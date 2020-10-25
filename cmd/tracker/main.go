package main

import (
	"net"
	"os"
	"smux-datagram/mux"
	"smux-datagram/protocol"
	"smux-datagram/tracker"
	"smux-datagram/util"

	log "github.com/sirupsen/logrus"
)

func main() {
	addr := "0.0.0.0:7500"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}
	if err := startTracker(addr); err != nil {
		log.Fatal(err)
	}
}

func startTracker(addr string) error {
	priv, err := protocol.ReadPrivateKey(nil)
	if err != nil {
		return err
	}
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	m, err := mux.New(laddr, priv)
	if err != nil {
		return err
	}
	defer m.Close()
	go tracker.Serve(m)
	log.Infof("Tracker serving at %s", m.Addr())
	log.Infof("Received signal %+v", util.WaitForSignal())
	return nil
}

func init() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
