package main

import (
	"net"
	"os"
	"smux-datagram/mux"
	"smux-datagram/peer"
	"smux-datagram/protocol"
	"smux-datagram/util"

	log "github.com/sirupsen/logrus"
)

func main() {
	addr := "127.0.0.1:7500"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	if err := startPeer(addr); err != nil {
		log.Fatal(err)
	}
}

func startPeer(addr string) error {
	priv, err := protocol.ReadPrivateKey(nil)
	if err != nil {
		return err
	}
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	log.Infof("Contacting tracker at %s", raddr)
	m, err := mux.New(nil, priv)
	if err != nil {
		return err
	}
	p := peer.New(m)
	message := make([]byte, 65535)
	if err := p.Start(raddr, message); err != nil {
		return err
	}
	defer p.Stop()
	log.Infof("Received signal %+v", util.WaitForSignal())
	return nil
}

func init() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
