package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrame(t *testing.T) {
	require := require.New(t)
	expected := NewRaw(FrameRaw, []byte("Hello, world!"))
	buf := &bytes.Buffer{}
	require.Nil(WriteFrame(buf, expected))
	actual, err := ReadFrame(buf)
	require.Nil(err)
	require.Equal(expected, actual)
}
