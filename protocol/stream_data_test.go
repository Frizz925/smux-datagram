package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamData(t *testing.T) {
	require := require.New(t)
	expected := NewStreamData(1, 0, []byte("Hello, world!"))
	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	actual, err := ReadStreamData(buf)
	require.Nil(err)
	require.Equal(expected, actual)
}

func TestStreamDataAck(t *testing.T) {
	require := require.New(t)
	expected := NewStreamDataAck(1, 0)
	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	actual, err := ReadStreamDataAck(buf)
	require.Nil(err)
	require.Equal(expected, actual)
}

func TestStreamDataFin(t *testing.T) {
	require := require.New(t)
	expected := NewStreamDataFin(1, 13)
	buf := &bytes.Buffer{}
	require.Nil(expected.Serialize(buf))
	actual, err := ReadStreamDataFin(buf)
	require.Nil(err)
	require.Equal(expected, actual)
}
