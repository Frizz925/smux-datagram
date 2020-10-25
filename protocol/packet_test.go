package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPacket(t *testing.T) {
	require := require.New(t)
	expected := Packet{
		ConnectionID: 1,
		Sequence:     1,
		Type:         PacketHandshake,
		Frame:        NewRaw(FrameRaw, []byte("Hello, world!")),
	}
	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	actual, err := Deserialize(buf)
	require.Nil(err)
	require.Equal(expected, actual)
}
