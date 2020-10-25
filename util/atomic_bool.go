package util

import "sync/atomic"

type AtomicBool struct {
	v int32
}

const (
	atomicFalse = int32(iota)
	atomicTrue
)

func (a *AtomicBool) Get() bool {
	return atomic.LoadInt32(&a.v) == atomicTrue
}

func (a *AtomicBool) Set(v bool) {
	flag := atomicFalse
	if v {
		flag = atomicTrue
	}
	atomic.StoreInt32(&a.v, flag)
}
