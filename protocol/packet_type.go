package protocol

import "io"

type PacketType uint8

const (
	PacketNone PacketType = iota
	PacketHandshake
	PacketPing
	PacketPong
	PacketTerminate
	PacketStream
)

func ReadPacketType(r io.Reader) (PacketType, error) {
	b, err := ReadByte(r)
	if err != nil {
		return 0, err
	}
	return PacketType(b), nil
}

func (pt PacketType) Serialize(w io.Writer) error {
	return WriteByte(w, byte(pt))
}
