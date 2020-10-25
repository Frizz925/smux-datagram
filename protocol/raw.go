package protocol

import (
	"bytes"
	"fmt"
	"io"
)

type Raw struct {
	ft      FrameType
	content []byte
}

func NewRaw(ft FrameType, b []byte) Raw {
	return Raw{
		ft:      ft,
		content: b,
	}
}

func ReadRaw(r io.Reader) ([]byte, error) {
	fn, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if fn > MaxPacketSize {
		return nil, bytes.ErrTooLarge
	}
	return ReadBytes(r, int(fn))
}

func WriteRaw(w io.Writer, b []byte) error {
	fn := uint(len(b))
	if fn > MaxPacketSize {
		return bytes.ErrTooLarge
	}
	if err := WriteVarInt(w, fn); err != nil {
		return err
	}
	return WriteFull(w, b)
}

func (r Raw) Type() FrameType {
	return r.ft
}

func (r Raw) Content() []byte {
	return r.content
}

func (r Raw) Reader() io.Reader {
	return bytes.NewReader(r.content)
}

func (r Raw) Serialize(w io.Writer) error {
	return WriteRaw(w, r.content)
}

func (r Raw) String() string {
	return fmt.Sprintf("Raw(Type: %d, Length: %d)", r.ft, len(r.content))
}
