package protocol

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandshake(t *testing.T) {
	require := require.New(t)
	randGen := rand.New(rand.NewSource(0))

	privKey, err := ReadPrivateKey(randGen)
	require.Nil(err)
	pubKey, err := privKey.PublicKey()
	require.Nil(err)

	expected := Handshake{
		FrameType:  FrameHandshake,
		BufferSize: 1024,
		PublicKey:  pubKey,
	}
	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	actual, err := ReadHandshake(buf, expected.FrameType)
	require.Nil(err)
	// Skip check for maxframesize
	expected.MaxFrameSize = actual.MaxFrameSize
	require.Equal(expected, actual)
}
