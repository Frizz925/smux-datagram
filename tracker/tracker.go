package tracker

import (
	"fmt"
	"net"
	"smux-datagram/util"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type SubscriberFunc func(addr string)

type Tracker struct {
	observable *util.Observable
	peers      map[string]net.Conn
	mu         sync.RWMutex
}

func New() *Tracker {
	return &Tracker{
		observable: util.NewObservable(),
		peers:      make(map[string]net.Conn),
	}
}

func (t *Tracker) Serve(l net.Listener) error {
	ob := t.observable.Observe(func(_ *util.Observer, v interface{}) {
		addr, ok := v.(string)
		if !ok {
			return
		}
		t.mu.RLock()
		defer t.mu.RUnlock()
		for paddr, conn := range t.peers {
			if paddr == addr {
				continue
			}
			go t.notifyPeer(conn, addr)
		}
	})
	defer ob.Remove()
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		log.Infof("Received connection from %s", conn.RemoteAddr())
		go t.clientWorker(conn)
	}
}

func (t *Tracker) clientWorker(conn net.Conn) {
	if err := t.handleClient(conn); err != nil {
		log.Errorf("Client error: %+v", err)
	}
}

func (t *Tracker) handleClient(conn net.Conn) error {
	defer func() {
		t.mu.Lock()
		saddr := conn.RemoteAddr().String()
		delete(t.peers, saddr)
		t.mu.Unlock()
	}()

	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		b := t.handlePacket(conn, buf[:n])
		if _, err := conn.Write(b); err != nil {
			return err
		}
	}
}

func (t *Tracker) handlePacket(conn net.Conn, b []byte) []byte {
	method := strings.SplitN(string(b), " ", 2)[0]
	resp, err := t.handleRequest(conn, method)
	if err != nil {
		resp = fmt.Sprintf("error %s", err)
	}
	return []byte(resp)
}

func (t *Tracker) handleRequest(conn net.Conn, method string) (string, error) {
	addr := conn.RemoteAddr().String()
	log.Infof("Handling request from %s: %s", addr, method)
	result := ""
	switch method {
	case "subscribe":
		t.mu.RLock()
		peers := make([]string, 0)
		for paddr := range t.peers {
			if addr != paddr {
				peers = append(peers, paddr)
			}
		}
		t.mu.RUnlock()
		if _, ok := t.peers[addr]; !ok {
			t.mu.Lock()
			t.peers[addr] = conn
			t.mu.Unlock()
			t.observable.Update(addr)
		}
		result = strings.Join(peers, " ")
	case "keepalive":
		// Do nothing
	default:
		return "", fmt.Errorf("unknown method: %s", method)
	}
	if result != "" {
		return fmt.Sprintf("ok %s", result), nil
	} else {
		return "ok", nil
	}
}

func (t *Tracker) notifyPeer(conn net.Conn, paddr string) {
	addr := conn.RemoteAddr()
	log.Infof("Notifying peer at %s of new peer: %s", addr, paddr)
	cmd := fmt.Sprintf("notify %s", paddr)
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		log.Errorf("Error notifying peer %s: %+v", addr, err)
	}
}

func Serve(l net.Listener) error {
	return New().Serve(l)
}
