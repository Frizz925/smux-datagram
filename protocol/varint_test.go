package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

type variableLengthTestCase struct {
	expected    uint
	expectedLen int
}

func TestVarInt(t *testing.T) {
	require := require.New(t)
	testCases := []variableLengthTestCase{
		{maxUint6, 1},
		{maxUint14, 2},
		{maxUint30, 4},
		{maxUint62, 8},
	}

	for _, tc := range testCases {
		buf := &bytes.Buffer{}
		require.Nil(WriteVarInt(buf, tc.expected))
		b := buf.Bytes()
		require.Equal(tc.expectedLen, len(b))

		actual, err := ReadVarInt(bytes.NewReader(b))
		require.Nil(err)
		require.Equal(tc.expected, actual)
	}
}
