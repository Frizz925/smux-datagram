package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObservable(t *testing.T) {
	require := require.New(t)
	expected := "Hello, world!"
	observable := NewObservable()

	ch := make(chan string)
	observable.Observe(func(o *Observer, v interface{}) {
		s, ok := v.(string)
		if ok {
			o.Remove()
			ch <- s
		}
	})
	observable.Update(expected)
	require.Equal(expected, <-ch)
}
