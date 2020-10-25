package protocol

import (
	"fmt"
	"io"
)

type StreamData struct {
	StreamFrame
	Offset uint32
	Data   []byte
}

func NewStreamData(sid StreamID, offset uint32, data []byte) StreamData {
	return StreamData{
		StreamFrame: NewStreamFrame(FrameStreamData, sid),
		Offset:      offset,
		Data:        data,
	}
}

func ReadStreamData(r io.Reader) (sd StreamData, err error) {
	var length uint
	sd.StreamFrame, err = ReadStreamFrame(r, FrameStreamData)
	if err != nil {
		return
	}
	sd.Offset, err = ReadUint32(r)
	if err != nil {
		return
	}
	length, err = ReadVarInt(r)
	if err != nil {
		return
	}
	sd.Data, err = ReadBytes(r, int(length))
	return sd, err
}

func (sd StreamData) Serialize(w io.Writer) error {
	if err := sd.StreamFrame.Serialize(w); err != nil {
		return err
	}
	if err := WriteUint32(w, sd.Offset); err != nil {
		return err
	}
	dlen := len(sd.Data)
	if err := WriteVarInt(w, uint(dlen)); err != nil {
		return err
	}
	return WriteFull(w, sd.Data)
}

func (sd StreamData) String() string {
	return fmt.Sprintf(
		"StreamData(%+v, Offset: %d, Length: %d)",
		sd.StreamFrame, sd.Offset, len(sd.Data),
	)
}

type StreamDataAck struct {
	StreamFrame
	Offset uint32
}

func NewStreamDataAck(sid StreamID, offset uint32) StreamDataAck {
	return StreamDataAck{
		StreamFrame: NewStreamFrame(FrameStreamDataAck, sid),
		Offset:      offset,
	}
}

func ReadStreamDataAck(r io.Reader) (sda StreamDataAck, err error) {
	sda.StreamFrame, err = ReadStreamFrame(r, FrameStreamDataAck)
	if err != nil {
		return
	}
	sda.Offset, err = ReadUint32(r)
	if err != nil {
		return
	}
	return sda, nil
}

func (sda StreamDataAck) Serialize(w io.Writer) error {
	if err := sda.StreamFrame.Serialize(w); err != nil {
		return err
	}
	return WriteUint32(w, sda.Offset)
}

func (sda StreamDataAck) String() string {
	return fmt.Sprintf("StreamDataAck(%+v, Offset: %d)", sda.StreamFrame, sda.Offset)
}

type StreamDataFin struct {
	StreamFrame
	Length uint
}

func NewStreamDataFin(sid StreamID, length uint) StreamDataFin {
	return StreamDataFin{
		StreamFrame: NewStreamFrame(FrameStreamDataFin, sid),
		Length:      length,
	}
}

func ReadStreamDataFin(r io.Reader) (sdf StreamDataFin, err error) {
	sdf.StreamFrame, err = ReadStreamFrame(r, FrameStreamDataFin)
	if err != nil {
		return
	}
	sdf.Length, err = ReadVarInt(r)
	return sdf, err
}

func (sdf StreamDataFin) Serialize(w io.Writer) error {
	if err := sdf.StreamFrame.Serialize(w); err != nil {
		return err
	}
	return WriteVarInt(w, sdf.Length)
}

func (sdf StreamDataFin) String() string {
	return fmt.Sprintf("StreamDataFin(%+v, Length: %d)", sdf.StreamFrame, sdf.Length)
}
