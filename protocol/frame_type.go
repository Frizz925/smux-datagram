package protocol

import "io"

type FrameType uint8

const (
	FrameUnknown FrameType = iota
	FrameRaw
	FrameCrypto
	FrameHandshake
	FrameStreamOpen
	FrameStreamAck
	FrameStreamReset
	FrameStreamData
	FrameStreamDataAck
	FrameStreamDataFin
	FrameStreamClose
)

func ReadFrameType(r io.Reader) (FrameType, error) {
	b, err := ReadByte(r)
	if err != nil {
		return 0, err
	}
	return FrameType(b), nil
}

func (ft FrameType) Serialize(w io.Writer) error {
	return WriteByte(w, byte(ft))
}
