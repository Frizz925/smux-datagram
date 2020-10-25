package mux

import (
	"fmt"
	"smux-datagram/protocol"
)

// DeferCrypto is a partial frame which carries unencrypted frame, which defers it for encryption.
// Due to its partial nature, its serialize function would result in serialization of the unencrypted frame.
// DeferCrypto is simply a type that helps serializer routine to check whether a frame should be encrypted during the serialization process.
type DeferCrypto struct {
	protocol.Frame
}

func WithDeferCrypto(frame protocol.Frame) DeferCrypto {
	return DeferCrypto{frame}
}

func (dc DeferCrypto) String() string {
	return fmt.Sprintf("DeferCrypto(%+v)", dc.Frame)
}
