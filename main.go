package main

import (
	"fmt"
	"net"
	"smux-datagram/mux"
	"smux-datagram/protocol"
	"smux-datagram/util"

	log "github.com/sirupsen/logrus"
)

func main() {
	if err := start(); err != nil {
		log.Fatal(err)
	}
}

func start() error {
	ms, err := startServer(nil)
	if err != nil {
		return err
	}
	defer ms.Close()
	go listenWorker(ms)

	port := ms.Addr().(*net.UDPAddr).Port
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	mc, err := startClient()
	if err != nil {
		return err
	}
	defer mc.Close()
	clientWorker(mc, raddr)

	log.Infof("Received signal %+v", util.WaitForSignal())

	return nil
}

func startServer(laddr *net.UDPAddr) (*mux.Mux, error) {
	priv, _, err := generateKeyPair()
	if err != nil {
		return nil, err
	}
	m, err := mux.New(laddr, priv)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func listenWorker(m *mux.Mux) {
	for {
		session, err := m.AcceptSession()
		if err != nil {
			log.Errorf("Listen error: %+v", err)
			return
		}
		go sessionWorker(session)
	}
}

func sessionWorker(session *mux.Session) {
	conn, err := session.Accept()
	if err != nil {
		log.Errorf("Session error: %+v", err)
		return
	}
	go streamWorker(conn)
}

func streamWorker(conn *mux.Stream) {
	if err := serve(conn); err != nil {
		log.Errorf("Serve error: %+v", err)
	}
}

func serve(conn net.Conn) error {
	buf := make([]byte, 65535)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	message := buf[:n]
	log.Infof("Server received: %s", string(message))
	log.Infof("Server sending: %s", string(message))
	if _, err := conn.Write(message); err != nil {
		return err
	}
	return nil
}

func startClient() (*mux.Mux, error) {
	priv, _, err := generateKeyPair()
	if err != nil {
		return nil, err
	}
	m, err := mux.New(nil, priv)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func clientWorker(m *mux.Mux, raddr *net.UDPAddr) {
	if err := runClient(m, raddr); err != nil {
		log.Errorf("Client error: %+v", err)
	}
}

func runClient(m *mux.Mux, raddr *net.UDPAddr) error {
	session, err := m.OpenSession(raddr)
	if err != nil {
		return err
	}
	conn, err := session.OpenStream()
	if err != nil {
		return err
	}

	message := "Hello, world!"
	log.Infof("Client sending: %s", message)
	if _, err := conn.Write([]byte(message)); err != nil {
		return err
	}

	b := make([]byte, 65535)
	n, err := conn.Read(b)
	if err != nil {
		return err
	}
	message = string(b[:n])
	log.Infof("Client received: %s", message)

	return nil
}

func generateKeyPair() (priv protocol.PrivateKey, pub protocol.PublicKey, err error) {
	priv, err = protocol.ReadPrivateKey(nil)
	if err != nil {
		return
	}
	pub, err = priv.PublicKey()
	if err != nil {
		return
	}
	return priv, pub, nil
}

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
