package util

import (
	"os"
	"os/signal"
	"syscall"
)

func WaitForSignal() os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	return <-ch
}
