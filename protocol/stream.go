package protocol

import (
	"fmt"
	"io"
)

type StreamID uint32

func ReadStreamID(r io.Reader) (StreamID, error) {
	sid, err := ReadUint32(r)
	if err != nil {
		return 0, err
	}
	return StreamID(sid), nil
}

func (sid StreamID) Serialize(w io.Writer) error {
	return WriteUint32(w, uint32(sid))
}

func (sid StreamID) String() string {
	return fmt.Sprintf("%d", sid)
}

type StreamFrame interface {
	Frame
	StreamID() StreamID
}

type baseStreamFrame struct {
	ft  FrameType
	sid StreamID
}

func NewStreamFrame(ft FrameType, sid StreamID) StreamFrame {
	return baseStreamFrame{
		sid: sid,
		ft:  ft,
	}
}

func ReadStreamFrame(r io.Reader, ft FrameType) (StreamFrame, error) {
	sid, err := ReadStreamID(r)
	if err != nil {
		return nil, err
	}
	return NewStreamFrame(ft, sid), nil
}

func (bsf baseStreamFrame) Type() FrameType {
	return bsf.ft
}

func (bsf baseStreamFrame) StreamID() StreamID {
	return bsf.sid
}

func (bsf baseStreamFrame) Serialize(w io.Writer) error {
	return bsf.sid.Serialize(w)
}

func (bsf baseStreamFrame) String() string {
	return fmt.Sprintf("StreamFrame(StreamID: %d, FrameType: %+v)", bsf.sid, bsf.ft)
}
