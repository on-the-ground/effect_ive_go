package state

import (
	"sync"
)

type inMemStore[K comparable] struct {
	*sync.Map
}

func (t inMemStore[K]) CompareAndDelete(k K, old any) bool {
	return t.Map.CompareAndDelete(k, old)
}

func (t inMemStore[K]) CompareAndSwap(k K, old, new any) bool {
	return t.Map.CompareAndSwap(k, old, new)
}

func (t inMemStore[K]) Load(k K) (any, bool) {
	return t.Map.Load(k)
}

func NewInMemoryStore[K comparable]() StateStore {
	return NewCasStore(inMemStore[K]{Map: &sync.Map{}})
}

func (t inMemStore[K]) InsertIfAbsent(key K, value any) (inserted bool) {
	_, loaded := t.LoadOrStore(key, value)
	return !loaded
}
