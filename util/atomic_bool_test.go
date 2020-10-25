package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicBool(t *testing.T) {
	require := require.New(t)
	val := AtomicBool{}
	val.Set(false)
	require.False(val.Get())
	val.Set(true)
	require.True(val.Get())
}
