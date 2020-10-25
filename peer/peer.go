package peer

import (
	"errors"
	"net"
	"smux-datagram/interop"
	"smux-datagram/mux"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const LoopInterval = 15 * time.Second

var ErrInterrupted = errors.New("interrupted")

type Peer struct {
	mux *mux.Mux

	message []byte

	mu sync.RWMutex
	wg *sync.WaitGroup

	notify chan string
	die    chan struct{}
}

// Peer fully controls the mux and its lifecycle.
// The mux should not be used again after being used by peer.
func New(m *mux.Mux) *Peer {
	return &Peer{
		mux:    m,
		wg:     &sync.WaitGroup{},
		notify: make(chan string),
		die:    make(chan struct{}),
	}
}

func (p *Peer) Start(raddr *net.UDPAddr, message []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.message = message
	p.wg.Add(3)
	go p.subscribeRoutine(raddr)
	go p.consumerRoutine()
	go p.listenRoutine()

	return nil
}

func (p *Peer) Stop() error {
	if err := p.mux.Close(); err != nil {
		return err
	}
	close(p.die)
	p.wg.Wait()
	return nil
}

func (p *Peer) subscribeRoutine(raddr *net.UDPAddr) {
	defer p.wg.Done()
	if err := p.subscribe(raddr); err != nil {
		log.Errorf("Subscriber error: %+v", err)
	}
}

func (p *Peer) subscribe(raddr *net.UDPAddr) error {
	session, err := p.mux.OpenSession(raddr)
	if err != nil {
		return err
	}
	defer session.Close()
	conn, err := session.OpenStream()
	if err != nil {
		return err
	}
	cmd := interop.New(conn)
	res, err := cmd.Call("subscribe")
	if err != nil {
		return err
	}
	addrs := strings.Split(res, " ")
	for _, addr := range addrs {
		p.notify <- addr
	}
	return cmd.Subscribe(func(method string, result string) {
		if method == "notify" && result != "" {
			p.notify <- result
		}
	})
}

func (p *Peer) consumerRoutine() {
	defer p.wg.Done()
	for {
		select {
		case addr := <-p.notify:
			if addr == "" {
				continue
			}
			log.Infof("Notified of new peer address: %s", addr)
			p.wg.Add(1)
			go p.messageRoutine(addr)
		case <-p.die:
			log.Errorf("Consumer error: %+v", ErrInterrupted)
			return
		}
	}
}

func (p *Peer) messageRoutine(addr string) {
	defer p.wg.Done()
	if err := p.sendMessage(addr); err != nil {
		log.Errorf("Peer error %s: %+v", addr, err)
	}
}

func (p *Peer) sendMessage(addr string) error {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	session, err := p.mux.OpenSession(raddr)
	if err != nil {
		return err
	}
	defer session.Close()
	conn, err := session.OpenStream()
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, 65535)
	for {
		w, err := conn.Write(p.message)
		if err != nil {
			return err
		}
		log.Infof("Sent message to peer at %s, length: %d", addr, w)
		r, err := conn.Read(buf)
		if err != nil {
			return err
		}
		log.Infof("Received message from peer at %s, length: %d", addr, r)
		time.Sleep(LoopInterval)
	}
}

func (p *Peer) listenRoutine() {
	defer p.wg.Done()
	for {
		conn, err := p.mux.Accept()
		if err != nil {
			log.Errorf("Listen error: %+v", err)
			return
		}
		log.Infof("Accepted new peer: %s", conn.RemoteAddr())
		p.wg.Add(1)
		go p.peerRoutine(conn)
	}
}

func (p *Peer) peerRoutine(conn net.Conn) {
	defer p.wg.Done()
	if err := p.handlePeer(conn); err != nil {
		log.Errorf("Peer error %s: %+v", conn.RemoteAddr(), err)
	}
}

func (p *Peer) handlePeer(conn net.Conn) error {
	defer conn.Close()
	buf := make([]byte, 65535)
	for {
		r, err := conn.Read(buf)
		if err != nil {
			return err
		}
		log.Infof("Received message from peer at %s, length: %d", conn.RemoteAddr(), r)
		w, err := conn.Write([]byte(p.message))
		if err != nil {
			return err
		}
		log.Infof("Sent message to peer at %s, length: %d", conn.RemoteAddr(), w)
		time.Sleep(LoopInterval)
	}
}

/*
func promptForInputs() (*net.UDPAddr, string, error) {
	con := bufio.NewReadWriter(
		bufio.NewReader(os.Stdin),
		bufio.NewWriter(os.Stdout),
	)
	addr := prompt(con, "Enter the tracker address: ", "127.0.0.1:7500")
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, "", err
	}
	message := prompt(con, "Enter the message to send to your peers: ", "Hello, world!")
	return raddr, message, nil
}

func prompt(con *bufio.ReadWriter, message string, fallback string) string {
	if _, err := con.WriteString(message); err != nil {
		return fallback
	}
	if err := con.Flush(); err != nil {
		return fallback
	}
	read, err := con.ReadString('\n')
	if err != nil {
		con.WriteString(fallback + "\n")
		con.Flush()
		return fallback
	}
	return strings.TrimSpace(read)
}
*/
