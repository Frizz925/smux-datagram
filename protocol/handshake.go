package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type Handshake struct {
	FrameType
	BufferSize uint
	PublicKey  PublicKey
	// This field is only filled when reading from byte stream.
	// This works by counting the size of handshake's padding.
	MaxFrameSize int
}

func ReadHandshake(r io.Reader, ft FrameType) (hs Handshake, err error) {
	hs.FrameType = ft
	hs.BufferSize, err = ReadVarInt(r)
	if err != nil {
		return
	}
	hs.PublicKey, err = ReadPublicKey(r)
	if err != nil {
		return
	}
	b := make([]byte, MaxPacketSize)
	// Read the first 32 bytes for overhead
	_, err = r.Read(b[:32])
	if err != nil {
		return
	}
	// Read the rest of the padding to determine the maximum frame size
	hs.MaxFrameSize, err = r.Read(b)
	return hs, err
}

func (hs Handshake) Type() FrameType {
	return hs.FrameType
}

func (hs Handshake) Serialize(w io.Writer) error {
	if err := WriteVarInt(w, hs.BufferSize); err != nil {
		return err
	}
	if err := hs.PublicKey.Serialize(w); err != nil {
		return err
	}
	paddingSize := MaxPacketSize
	buf, ok := w.(*bytes.Buffer)
	if ok {
		paddingSize -= buf.Len()
	}
	return WriteFull(w, make([]byte, paddingSize))
}

func (hs Handshake) String() string {
	return fmt.Sprintf(
		"Handshake(Type: %d, BufferSize: %d, MaxFrameSize: %d, PublicKey: %s)",
		hs.FrameType, hs.BufferSize, hs.MaxFrameSize, hs.PublicKey,
	)
}
