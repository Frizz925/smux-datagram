package mux

import (
	"errors"
	"fmt"
	"net"
	"smux-datagram/protocol"
	"smux-datagram/util"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// TODO: Make these configureable
const (
	pingInterval      = 30 * time.Second
	pingRetryMax      = 5
	pingRetryInterval = pingInterval / pingRetryMax

	// Size of the sliding window for keeping track of packet sequences.
	// Must be divisable by 2.
	ackWindowSize = 64

	// Used to keep track of idle connections. Connections that are idle for
	// more than this value would be terminated.
	idleTimeout = 120 * time.Second
)

var (
	ErrSessionClosed      = errors.New("session closed")
	ErrSessionInterrupted = errors.New("session interrupted")
)

type SessionConfig struct {
	cid          protocol.ConnectionID
	key          []byte
	maxFrameSize int
	bufferSize   int
	rtt          uint64
	timestamp    time.Time
}

type Session struct {
	mu sync.Mutex

	cfg SessionConfig
	cid protocol.ConnectionID

	mux    *Mux
	crypto *protocol.CryptoFacade
	seq    uint32

	streams struct {
		sync.RWMutex
		idMap   map[protocol.StreamID]*Stream
		openMap map[protocol.StreamID]chan<- *Stream
		id      uint32
	}

	stats struct {
		sync.RWMutex
		offset uint32
		window []uint32

		rtt      uint64
		lastRecv time.Time
		lastSend time.Time
	}

	routines struct {
		sync.WaitGroup
		ping    chan protocol.Packet
		inbound chan protocol.Packet
	}

	accept chan *Stream
	die    chan struct{}

	closed util.AtomicBool
}

func NewSession(mux *Mux, cfg SessionConfig) (*Session, error) {
	aead, err := chacha20poly1305.New(cfg.key)
	if err != nil {
		return nil, err
	}
	crypto := protocol.NewCryptoFacade(aead)
	s := &Session{
		cfg:    cfg,
		cid:    cfg.cid,
		mux:    mux,
		crypto: crypto,
	}

	s.streams.idMap = make(map[protocol.StreamID]*Stream)
	s.streams.openMap = make(map[protocol.StreamID]chan<- *Stream)

	s.stats.window = make([]uint32, ackWindowSize)
	s.stats.rtt = cfg.rtt

	s.routines.inbound = make(chan protocol.Packet, InboundBufferSize)
	s.routines.ping = make(chan protocol.Packet, 1)

	s.accept = make(chan *Stream)
	s.die = make(chan struct{})

	s.routines.Add(1)
	go s.inboundRoutine()

	log.Debugf("Created: %+v", s)
	s.closed.Set(false)
	return s, nil
}

func (s *Session) OpenStream() (*Stream, error) {
	if s.closed.Get() {
		return nil, ErrSessionClosed
	}

	var sid protocol.StreamID
	s.streams.RLock()
	for {
		sid = protocol.StreamID(s.nextId())
		_, ok := s.streams.idMap[sid]
		if !ok {
			break
		}
	}
	s.streams.RUnlock()

	ch := make(chan *Stream)
	s.streams.Lock()
	s.streams.openMap[sid] = ch
	s.streams.Unlock()

	defer func() {
		s.streams.Lock()
		delete(s.streams.openMap, sid)
		s.streams.Unlock()
		close(ch)
	}()

	openFrame := protocol.NewStreamFrame(protocol.FrameStreamOpen, sid)
	if err := s.SendStream(openFrame); err != nil {
		return nil, err
	}

	select {
	case conn := <-ch:
		return conn, nil
	case <-s.die:
		return nil, ErrSessionInterrupted
	}
}

func (s *Session) Accept() (*Stream, error) {
	if s.closed.Get() {
		return nil, ErrSessionClosed
	}
	select {
	case conn := <-s.accept:
		return conn, nil
	case <-s.die:
		return nil, ErrSessionInterrupted
	}
}

func (s *Session) LocalAddr() net.Addr {
	return s.mux.Addr()
}

func (s *Session) RemoteAddr() *net.UDPAddr {
	return s.mux.PeerAddr(s.cid)
}

func (s *Session) RTT() time.Duration {
	rtt := atomic.LoadUint64(&s.stats.rtt)
	if rtt <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(rtt)
}

func (s *Session) Send(packetType protocol.PacketType, frame protocol.Frame) error {
	if s.closed.Get() {
		return ErrSessionClosed
	}
	return s.mux.Send(protocol.Packet{
		ConnectionID: s.cid,
		Sequence:     s.nextSeq(),
		Type:         packetType,
		Frame:        frame,
	})
}

func (s *Session) SendStream(frame protocol.Frame) error {
	if s.closed.Get() {
		return ErrSessionClosed
	}
	return s.Send(protocol.PacketStream, frame)
}

func (s *Session) Close() error {
	return s.close(true)
}

func (s *Session) String() string {
	return fmt.Sprintf("Session(CID: %d, Addr: %s, %+v)", s.cid, s.RemoteAddr(), s.mux)
}

func (s *Session) inboundRoutine() {
	defer s.routines.Done()
	idleTicker := time.NewTicker(idleTimeout)
	defer idleTicker.Stop()
	for {
		idleTicker.Reset(idleTimeout)
		select {
		case packet := <-s.routines.inbound:
			s.dispatch(packet)
		case <-idleTicker.C:
			log.Warnf("%s: Connection idle reached max", s)
			go s.close(true)
			return
		case <-s.die:
			return
		}
	}
}

func (s *Session) nextSeq() uint32 {
	return atomic.AddUint32(&s.seq, 1)
}

func (s *Session) nextId() uint32 {
	return atomic.AddUint32(&s.streams.id, 1)
}

func (s *Session) decrypt(crypto protocol.Crypto, nonce protocol.Nonce) (protocol.Frame, error) {
	return s.crypto.Decrypt(crypto, nonce)
}

func (s *Session) encrypt(frame protocol.Frame, nonce protocol.Nonce) (protocol.Crypto, error) {
	return s.crypto.Encrypt(frame, nonce)
}

func (s *Session) dispatch(packet protocol.Packet) {
	// Simple mechanism against replay attacks by keeping track received packet sequences.
	// It's using sliding window implementation where packet sequence lower than the
	// lowest window value would be discarded.
	offset := atomic.LoadUint32(&s.stats.offset)
	seq := packet.Sequence
	if seq < offset {
		return
	}

	s.stats.Lock()
	window := s.stats.window
	windowSize := uint32(len(window))
	halfSize := windowSize / 2
	index := seq - offset
	// In case of the received packet has sequence value higher than our highest window value
	// then we need to keep moving our window by half of its size until it fits.
	for index >= windowSize {
		offset = atomic.AddUint32(&s.stats.offset, halfSize)
		for i := uint32(0); i < halfSize; i++ {
			window[i] = window[i+halfSize]
			window[i+halfSize] = 0
		}
		index = seq - offset
	}
	// If we have seen this packet sequence before then discard it.
	if window[index] == seq {
		s.stats.Unlock()
		return
	}
	window[index] = seq
	s.stats.Unlock()

	// If the RTT is not known (zero) then count it based on our session timestamp
	rtt := atomic.LoadUint64(&s.cfg.rtt)
	if rtt <= 0 {
		ts := s.cfg.timestamp
		rtt = uint64(time.Now().UnixNano() - ts.UnixNano())
		atomic.StoreUint64(&s.cfg.rtt, rtt)
	}

	switch packet.Type {
	case protocol.PacketPing:
		s.sendPong()
	default:
		if packet.Frame != nil {
			s.dispatchFrame(packet.Frame)
		}
	}
}

func (s *Session) sendPong() {
	if err := s.Send(protocol.PacketPong, nil); err != nil {
		log.Errorf("%s: Pong error: %+v", s, err)
	}
}

func (s *Session) dispatchFrame(frame protocol.Frame) {
	switch v := frame.(type) {
	case protocol.StreamFrame:
		switch v.Type() {
		case protocol.FrameStreamOpen:
			s.openStream(v.StreamID(), false)
		case protocol.FrameStreamAck:
			s.openStream(v.StreamID(), true)
		case protocol.FrameStreamClose:
			s.closeStream(v.StreamID())
		default:
			s.streamDispatch(v.StreamID(), v)
		}
	}
}

func (s *Session) openStream(sid protocol.StreamID, isAck bool) {
	s.streams.RLock()
	_, ok := s.streams.idMap[sid]
	s.streams.RUnlock()
	if ok {
		return
	}

	conn := NewStream(s, StreamConfig{
		SessionConfig: s.cfg,
		sid:           sid,
	})

	s.streams.Lock()
	s.streams.idMap[sid] = conn
	if isAck {
		ch, ok := s.streams.openMap[sid]
		if ok {
			delete(s.streams.openMap, sid)
			ch <- conn
		}
		s.streams.Unlock()
		return
	}
	s.streams.Unlock()

	select {
	case s.accept <- conn:
	default:
	}

	// We need to send ACK frame back for peer-initiated stream
	s.SendStream(protocol.NewStreamFrame(protocol.FrameStreamAck, sid))
}

func (s *Session) streamDispatch(sid protocol.StreamID, frame protocol.Frame) {
	s.streams.RLock()
	conn, ok := s.streams.idMap[sid]
	s.streams.RUnlock()
	if ok {
		conn.dispatch(frame)
	}
}

func (s *Session) closeStream(sid protocol.StreamID) {
	s.streams.RLock()
	conn, ok := s.streams.idMap[sid]
	s.streams.RUnlock()
	if ok {
		conn.Close()
	}
}

func (s *Session) remove(sid protocol.StreamID) {
	s.streams.Lock()
	delete(s.streams.idMap, sid)
	delete(s.streams.openMap, sid)
	s.streams.Unlock()
}

func (s *Session) close(detach bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Get() {
		return ErrSessionClosed
	}
	s.closed.Set(true)
	log.Debugf("%+v: Closing", s)
	s.streams.RLock()
	for _, conn := range s.streams.idMap {
		if err := conn.close(false); err != nil {
			s.streams.RUnlock()
			return err
		}
	}
	s.streams.RUnlock()
	terminatePacket := protocol.Packet{
		ConnectionID: s.cid,
		Sequence:     s.nextSeq(),
		Type:         protocol.PacketTerminate,
	}
	if err := s.mux.Send(terminatePacket); err != nil {
		return err
	}
	close(s.die)
	if detach {
		s.mux.remove(s.cid)
	}
	s.routines.Wait()
	return nil
}
