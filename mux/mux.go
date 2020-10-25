package mux

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"reflect"
	"smux-datagram/protocol"
	"smux-datagram/util"
	"sync"
	"time"

	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/hkdf"
)

const (
	InboundBufferSize  = 1024
	OutboundBufferSize = 1024
	StreamBufferSize   = 65535
)

var (
	ErrMuxClosed        = errors.New("mux closed")
	ErrMuxInterrupted   = errors.New("mux interrupted")
	ErrHandshakeTimeout = errors.New("handshake timeout")
)

type Inbound struct {
	RemoteAddr *net.UDPAddr
	Buffer     []byte
	Packet     protocol.Packet
}

type Outbound struct {
	RemoteAddr *net.UDPAddr
	Buffer     []byte
	Packet     protocol.Packet
	Sent       chan struct{}
}

type HandshakeState struct {
	Ephemeral protocol.PrivateKey
	Result    chan<- *Session
	Timestamp time.Time
}

type Mux struct {
	conn *net.UDPConn
	hkdf io.Reader

	queue struct {
		deserialize chan Inbound
		demux       chan Inbound
		mux         chan Outbound
		serialize   chan Outbound
		outbound    chan Outbound
	}

	sessions struct {
		sync.RWMutex
		// Session mapping
		idMap map[protocol.ConnectionID]*Session
		// Remote address mapping
		addrMap map[protocol.ConnectionID]*net.UDPAddr
		// Handshake states
		hsMap map[protocol.ConnectionID]*HandshakeState
	}

	die    chan struct{}
	accept chan *Session
	wg     sync.WaitGroup
	mu     sync.Mutex

	closed util.AtomicBool
}

var _ net.Listener = (*Mux)(nil)

func New(laddr *net.UDPAddr, priv protocol.PrivateKey) (*Mux, error) {
	salt := make([]byte, protocol.KeySize)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %+v", err)
	}

	hkdf := hkdf.New(func() hash.Hash {
		h, _ := blake2s.New256(nil)
		return h
	}, priv[:], salt, nil)

	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, err
	}

	m := &Mux{
		conn: conn,
		hkdf: hkdf,

		die:    make(chan struct{}),
		accept: make(chan *Session),
	}

	// Receive flow: Inbound -> Deserialize -> Demux
	// Send flow: Mux -> Serialize -> Outbound
	m.queue.deserialize = make(chan Inbound, InboundBufferSize)
	m.queue.demux = make(chan Inbound, InboundBufferSize)
	m.queue.mux = make(chan Outbound, OutboundBufferSize)
	m.queue.serialize = make(chan Outbound, OutboundBufferSize)
	m.queue.outbound = make(chan Outbound, OutboundBufferSize)

	m.sessions.idMap = make(map[protocol.ConnectionID]*Session)
	m.sessions.addrMap = make(map[protocol.ConnectionID]*net.UDPAddr)
	m.sessions.hsMap = make(map[protocol.ConnectionID]*HandshakeState)

	m.wg.Add(4)
	go m.demuxRoutine()
	go m.deserializeRoutine()
	go m.serializeRoutine()
	go m.muxRoutine()

	m.wg.Add(2)
	go m.writeRoutine(conn)
	go m.readRoutine(conn)

	log.Debugf("Created: %+v", m)
	m.closed.Set(false)
	return m, nil
}

func (m *Mux) Addr() net.Addr {
	return m.conn.LocalAddr()
}

func (m *Mux) PeerAddr(cid protocol.ConnectionID) *net.UDPAddr {
	m.sessions.RLock()
	defer m.sessions.RUnlock()
	return m.sessions.addrMap[cid]
}

func (m *Mux) OpenSession(raddr *net.UDPAddr) (*Session, error) {
	if m.closed.Get() {
		return nil, ErrMuxClosed
	}
	cid, err := protocol.GenerateConnectionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate connection id: %+v", err)
	}

	priv, err := protocol.ReadPrivateKey(m.hkdf)
	if err != nil {
		return nil, fmt.Errorf("failed to create ephemeral secret: %+v", err)
	}
	pub, err := priv.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create ephemeral public secret: %+v", err)
	}

	ch := make(chan *Session)
	hs := &HandshakeState{
		Ephemeral: priv,
		Result:    ch,
		Timestamp: time.Now(),
	}
	m.sessions.Lock()
	m.sessions.addrMap[cid] = raddr
	m.sessions.hsMap[cid] = hs
	m.sessions.Unlock()

	defer func() {
		m.sessions.Lock()
		delete(m.sessions.hsMap, cid)
		m.sessions.Unlock()
		close(ch)
	}()

	// TODO: Make these configurable
	interval := 3 * time.Second
	timeout := 30 * time.Second

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for seq := uint32(0); ; seq++ {
		err := m.sendHandshake(cid, seq+1, pub)
		if err != nil {
			return nil, err
		}
		select {
		case <-ticker.C:
			continue
		case session := <-ch:
			return session, nil
		case <-timer.C:
			return nil, ErrHandshakeTimeout
		case <-m.die:
			return nil, ErrMuxInterrupted
		}
	}
}

func (m *Mux) AcceptSession() (*Session, error) {
	if m.closed.Get() {
		return nil, ErrMuxClosed
	}
	select {
	case session := <-m.accept:
		return session, nil
	case <-m.die:
		return nil, ErrMuxInterrupted
	}
}

func (m *Mux) Accept() (net.Conn, error) {
	if m.closed.Get() {
		return nil, ErrMuxClosed
	}
	session, err := m.AcceptSession()
	if err != nil {
		return nil, err
	}
	return session.Accept()
}

func (m *Mux) Send(packet protocol.Packet) error {
	if m.closed.Get() {
		return ErrMuxClosed
	}
	ch := make(chan struct{})
	m.queue.mux <- Outbound{
		Packet: packet,
		Sent:   ch,
	}
	select {
	case <-ch:
		return nil
	case <-m.die:
		return ErrMuxInterrupted
	}
}

func (m *Mux) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Get() {
		return ErrMuxClosed
	}
	m.closed.Set(true)
	log.Debugf("%+v: Closing", m)

	m.sessions.RLock()
	for _, session := range m.sessions.idMap {
		if err := session.close(false); err != nil {
			m.sessions.RUnlock()
			return err
		}
	}
	m.sessions.RUnlock()

	close(m.die)
	if err := m.conn.Close(); err != nil {
		return err
	}

	// Drain all queues
	draining := true
	for draining {
		select {
		case <-m.queue.deserialize:
		case <-m.queue.demux:
		case <-m.queue.serialize:
		case <-m.queue.outbound:
		default:
			draining = false
		}
	}

	m.wg.Wait()
	return nil
}

func (m *Mux) String() string {
	return fmt.Sprintf("Mux(Addr: %s)", m.Addr())
}

func (m *Mux) readRoutine(conn *net.UDPConn) {
	defer m.wg.Done()
	buf := protocol.NewBuffer()
	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		// If somehow we can't read from the connection then it's probably closed
		if err != nil {
			return
		}
		in := Inbound{
			RemoteAddr: raddr,
			Buffer:     make([]byte, n),
		}
		copy(in.Buffer, buf[:n])
		m.queue.deserialize <- in
	}
}

func (m *Mux) deserializeRoutine() {
	defer m.wg.Done()
	buf := bytes.NewBuffer(protocol.EmptyBuffer())
	for {
		select {
		case in := <-m.queue.deserialize:
			buf.Reset()
			buf.Write(in.Buffer)
			packet, err := protocol.Deserialize(buf)
			if err != nil {
				// Ignore failed deserialization
				continue
			}

			m.sessions.RLock()
			cid := packet.ConnectionID
			raddr, addrExists := m.sessions.addrMap[cid]
			session, sessionExists := m.sessions.idMap[cid]
			m.sessions.RUnlock()

			// Update the address map if it either doesn't exist
			// or has changed since it was last registered
			if !addrExists || raddr.String() != in.RemoteAddr.String() {
				m.sessions.Lock()
				m.sessions.addrMap[cid] = in.RemoteAddr
				m.sessions.Unlock()
			}

			// Decrypt if we have handshaked session and it's a crypto frame
			if sessionExists && packet.Frame != nil && packet.Frame.Type() == protocol.FrameCrypto {
				crypto, ok := packet.Frame.(protocol.Crypto)
				if !ok {
					// We shouldn't be able to get here
					// Since crypto frames should be deserialized into raw frames
					t := reflect.TypeOf(packet.Frame)
					log.Errorf(
						"Expected frame with crypto type to be deserialized into Crypto frame, got %s frame instead: %+v",
						t.Name(), packet.Frame,
					)
					continue
				}
				nonce := packet.Nonce()
				frame, err := session.decrypt(crypto, nonce)
				if err != nil {
					log.Errorf("Failed to decrypt crypto frame: %+v", err)
					continue
				}
				packet.Frame = frame
			}

			in.Packet = packet
			m.queue.demux <- in
		case <-m.die:
			return
		}
	}
}

func (m *Mux) demuxRoutine() {
	defer m.wg.Done()
	for {
		select {
		case in := <-m.queue.demux:
			packet := in.Packet
			cid := packet.ConnectionID
			log.Debugf("recv: %+v", packet)

			m.sessions.RLock()
			session, sessionExists := m.sessions.idMap[cid]
			m.sessions.RUnlock()
			if sessionExists {
				session.dispatch(packet)
				continue
			}

			hs, ok := packet.Frame.(protocol.Handshake)
			if ok {
				m.handleHandshake(cid, hs)
				continue
			}
		case <-m.die:
			return
		}
	}
}

func (m *Mux) muxRoutine() {
	defer m.wg.Done()
	for {
		select {
		case out := <-m.queue.mux:
			packet := out.Packet
			cid := packet.ConnectionID

			m.sessions.RLock()
			raddr, addrExists := m.sessions.addrMap[cid]
			m.sessions.RUnlock()
			if !addrExists {
				log.Errorf("Packet not sent due to missing address: %+v", packet)
				continue
			}
			out.RemoteAddr = raddr

			// Terminate session
			if packet.Type == protocol.PacketTerminate {
				go m.scheduleSessionRemoval(cid, out.Sent)
			}

			m.queue.serialize <- out
		case <-m.die:
			return
		}
	}
}

func (m *Mux) serializeRoutine() {
	defer m.wg.Done()
	buf := bytes.NewBuffer(protocol.EmptyBuffer())
	for {
		select {
		case out := <-m.queue.serialize:
			packet := out.Packet
			log.Debugf("send: %+v", packet)

			m.sessions.RLock()
			cid := packet.ConnectionID
			session, sessionExists := m.sessions.idMap[cid]
			m.sessions.RUnlock()

			frame := packet.Frame
			if v, ok := frame.(DeferCrypto); ok {
				if !sessionExists {
					// For security reasons we drop the outbound packet if somehow
					// we're trying to send packet with (to be) encrypted frame
					// but we haven't had handshaked with the peer yet
					log.Errorf("Packet with crypto frame has no associated session: %+v", packet)
					continue
				}
				nonce := packet.Nonce()
				cf, err := session.encrypt(v.Frame, nonce)
				if err != nil {
					log.Errorf("Failed to encrypt lazy crypto frame: %+v", err)
					continue
				}
				packet.Frame = cf
			}

			buf.Reset()
			if err := packet.Serialize(buf); err != nil {
				log.Errorf("Packet failed to serialize: %+v", packet)
				continue
			}
			n := buf.Len()
			out.Buffer = make([]byte, n)
			copy(out.Buffer, buf.Bytes())

			m.queue.outbound <- out
		case <-m.die:
			return
		}
	}
}

func (m *Mux) writeRoutine(conn *net.UDPConn) {
	defer m.wg.Done()
	for {
		select {
		case out := <-m.queue.outbound:
			_, err := conn.WriteToUDP(out.Buffer, out.RemoteAddr)
			// If somehow we can't write to the connection then it's probably closed
			if err != nil {
				return
			}
			close(out.Sent)
		case <-m.die:
			return
		}
	}
}

func (m *Mux) handleHandshake(cid protocol.ConnectionID, hs protocol.Handshake) {
	log.Debugf("Received handshake: CID(%d), %+v", cid, hs)

	m.sessions.RLock()
	state, stateExists := m.sessions.hsMap[cid]
	m.sessions.RUnlock()

	var priv protocol.PrivateKey
	rtt := uint64(0)
	ts := time.Now()
	if stateExists {
		priv = state.Ephemeral
		ts = state.Timestamp
		rtt = uint64(time.Now().UnixNano() - ts.UnixNano())
	} else {
		pk, err := protocol.ReadPrivateKey(m.hkdf)
		if err != nil {
			log.Errorf("Failed to create ephemeral secret: %+v", err)
			return
		}
		priv = pk
	}

	ss, err := priv.SharedSecret(hs.PublicKey)
	if err != nil {
		log.Errorf("Failed to create handshake shared secret: %+v", err)
		return
	}

	session, err := NewSession(m, SessionConfig{
		cid:          cid,
		key:          ss,
		bufferSize:   int(hs.BufferSize),
		maxFrameSize: hs.MaxFrameSize,
		rtt:          rtt,
		timestamp:    time.Now(),
	})
	if err != nil {
		log.Errorf("Failed to create session: %+v", err)
		return
	}

	m.sessions.Lock()
	m.sessions.idMap[cid] = session
	if stateExists {
		state.Result <- session
		m.sessions.Unlock()
		return
	}
	m.sessions.Unlock()

	// Notify accept caller of the new session
	select {
	case m.accept <- session:
	default:
	}

	// Create public key from our ephemeral secret
	pub, err := priv.PublicKey()
	if err != nil {
		log.Errorf("Failed to create ephemeral public key: %+v", err)
		return
	}

	// If the handshake packet we get is peer-initiated one, we need to respond with handshake ack
	if err := m.sendHandshake(cid, session.nextSeq(), pub); err != nil {
		log.Errorf("Failed to send handshake ack: %+v", err)
	}
}

func (m *Mux) sendHandshake(cid protocol.ConnectionID, seq uint32, pub protocol.PublicKey) error {
	hs := protocol.Handshake{
		BufferSize: StreamBufferSize,
		FrameType:  protocol.FrameHandshake,
		PublicKey:  pub,
	}
	packet := protocol.Packet{
		ConnectionID: cid,
		Sequence:     1,
		Type:         protocol.PacketHandshake,
		Frame:        hs,
	}
	return m.Send(packet)
}

func (m *Mux) scheduleSessionRemoval(cid protocol.ConnectionID, sent <-chan struct{}) {
	<-sent
	m.sessions.Lock()
	delete(m.sessions.addrMap, cid)
	m.sessions.Unlock()
}

func (m *Mux) remove(cid protocol.ConnectionID) {
	m.sessions.Lock()
	delete(m.sessions.idMap, cid)
	m.sessions.Unlock()
}
