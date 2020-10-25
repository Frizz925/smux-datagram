package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
)

func ReadFull(r io.Reader, b []byte) error {
	_, err := io.ReadFull(r, b)
	return err
}

func ReadByte(r io.Reader) (byte, error) {
	b := make([]byte, 1)
	return b[0], ReadFull(r, b)
}

func ReadBytes(r io.Reader, n int) ([]byte, error) {
	b := make([]byte, n)
	return b, ReadFull(r, b)
}

func WriteFull(w io.Writer, b []byte) error {
	off := 0
	for off < len(b) {
		n, err := w.Write(b[off:])
		if err != nil {
			return err
		}
		off += n
	}
	return nil
}

func WriteByte(w io.Writer, v byte) error {
	return WriteFull(w, []byte{v})
}

func CopyBuffer(buf *bytes.Buffer) []byte {
	b := make([]byte, buf.Len())
	copy(b, buf.Bytes())
	return b
}

func BytesToUint16(b []byte) (uint16, error) {
	if len(b) < 2 {
		return 0, io.ErrShortBuffer
	}
	return binary.BigEndian.Uint16(b), nil
}

func BytesToUint32(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, io.ErrShortBuffer
	}
	return binary.BigEndian.Uint32(b), nil
}

func BytesToUint64(b []byte) (uint64, error) {
	if len(b) < 8 {
		return 0, io.ErrShortBuffer
	}
	return binary.BigEndian.Uint64(b), nil
}

func BytesToUint(b []byte) uint {
	length := len(b)
	if length >= 8 {
		v, _ := BytesToUint64(b)
		return uint(v)
	} else if length >= 4 {
		v, _ := BytesToUint32(b)
		return uint(v)
	} else if length >= 2 {
		v, _ := BytesToUint16(b)
		return uint(v)
	} else if length >= 1 {
		return uint(b[0])
	}
	return 0
}

func Uint16ToBytes(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}

func Uint32ToBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func Uint64ToBytes(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func ReadUint16(r io.Reader) (uint16, error) {
	b, err := ReadBytes(r, 2)
	if err != nil {
		return 0, err
	}
	return BytesToUint16(b)
}

func ReadUint32(r io.Reader) (uint32, error) {
	b, err := ReadBytes(r, 4)
	if err != nil {
		return 0, err
	}
	return BytesToUint32(b)
}

func ReadUint64(r io.Reader) (uint64, error) {
	b, err := ReadBytes(r, 8)
	if err != nil {
		return 0, err
	}
	return BytesToUint64(b)
}

func WriteUint16(w io.Writer, v uint16) error {
	return WriteFull(w, Uint16ToBytes(v))
}

func WriteUint32(w io.Writer, v uint32) error {
	return WriteFull(w, Uint32ToBytes(v))
}

func WriteUint64(w io.Writer, v uint64) error {
	return WriteFull(w, Uint64ToBytes(v))
}
