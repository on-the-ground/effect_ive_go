package state

import (
	"sync"
)

type inMemStore[K comparable] struct {
	*sync.Map
}

func (t inMemStore[K]) CompareAndDelete(k K, old any) (ok bool, err error) {
	ok = t.Map.CompareAndDelete(k, old)
	return
}

func (t inMemStore[K]) CompareAndSwap(k K, old, new any) (ok bool, err error) {
	ok = t.Map.CompareAndSwap(k, old, new)
	return
}

func (t inMemStore[K]) Load(k K) (v any, ok bool, err error) {
	v, ok = t.Map.Load(k)
	return
}

func (t inMemStore[K]) InsertIfAbsent(k K, v any) (ok bool, err error) {
	_, loaded := t.Map.LoadOrStore(k, v)
	return !loaded, nil
}

func NewInMemoryStore[K comparable]() StateStore {
	return NewCasStore(inMemStore[K]{Map: &sync.Map{}})
}
