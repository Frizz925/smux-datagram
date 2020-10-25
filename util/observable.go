package util

import (
	"sync"
	"sync/atomic"
)

type ObserverFunc func(o *Observer, v interface{})

type Observable struct {
	mu        sync.RWMutex
	observers map[uint32]*Observer
	nextId    uint32
}

func NewObservable() *Observable {
	return &Observable{
		observers: make(map[uint32]*Observer),
	}
}

func (o *Observable) Observe(handler ObserverFunc) *Observer {
	id := atomic.AddUint32(&o.nextId, 1)
	observer := NewObserver(o, id, handler)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.observers[id] = observer
	return observer
}

func (o *Observable) Update(v interface{}) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, observer := range o.observers {
		go observer.update(v)
	}
}

func (o *Observable) remove(id uint32) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.observers, id)
}

type Observer struct {
	observable *Observable
	id         uint32
	handler    ObserverFunc
}

func NewObserver(o *Observable, id uint32, handler ObserverFunc) *Observer {
	return &Observer{
		observable: o,
		id:         id,
		handler:    handler,
	}
}

func (o *Observer) Remove() {
	o.observable.remove(o.id)
}

func (o *Observer) update(v interface{}) {
	o.handler(o, v)
}
