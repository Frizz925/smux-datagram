package protocol

import (
	"crypto/rand"
	"fmt"
	"io"
)

type ConnectionID uint64

func ReadConnectionID(r io.Reader) (ConnectionID, error) {
	sid, err := ReadUint64(r)
	if err != nil {
		return 0, err
	}
	return ConnectionID(sid), nil
}

func GenerateConnectionID() (ConnectionID, error) {
	return ReadConnectionID(rand.Reader)
}

func (cid ConnectionID) Serialize(w io.Writer) error {
	return WriteUint64(w, uint64(cid))
}

func (cid ConnectionID) String() string {
	return fmt.Sprintf("%d", cid)
}
