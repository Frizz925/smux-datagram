package protocol

import (
	"io"
)

type Serializer interface {
	Serialize(w io.Writer) error
}

type Frame interface {
	Serializer
	Type() FrameType
}

// ReadFrame may return nil Frame without error if it doesn't read any frame type.
func ReadFrame(r io.Reader) (Frame, error) {
	ft, err := ReadFrameType(r)
	if err != nil {
		// If no frame type is read then return immediately
		if err == io.EOF {
			return nil, nil
		} else {
			return nil, err
		}
	}

	switch ft {
	case FrameHandshake:
		return ReadHandshake(r, ft)
	case FrameStreamOpen:
		fallthrough
	case FrameStreamAck:
		fallthrough
	case FrameStreamReset:
		fallthrough
	case FrameStreamClose:
		return ReadStreamFrame(r, ft)
	case FrameStreamData:
		return ReadStreamData(r)
	case FrameStreamDataAck:
		return ReadStreamDataAck(r)
	case FrameStreamDataFin:
		return ReadStreamDataFin(r)
	case FrameCrypto:
		return ReadCrypto(r)
	default:
		b, err := ReadRaw(r)
		if err != nil {
			return nil, err
		}
		return NewRaw(ft, b), nil
	}
}

func WriteFrame(w io.Writer, f Frame) error {
	if err := f.Type().Serialize(w); err != nil {
		return err
	}
	return f.Serialize(w)
}
