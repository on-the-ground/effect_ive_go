package pure

import (
	"sync"
	"sync/atomic"
)

type Trie[O any] struct {
	memos   [2]*sync.Map
	headIdx uint32
	size    atomic.Uint32
	maxSize uint32
}

func (t *Trie[O]) Load(keys []ComparableOrString) (O, bool) {
	headIdx := t.headIdx
	targetMap := t.memos[headIdx]
	m, k := t.traverse(targetMap, keys)
	v, ok := m.Load(k)
	if !ok {
		targetMap = t.memos[1-headIdx]
		m, k := t.traverse(targetMap, keys)
		v, ok = m.Load(k)
		if !ok {
			var zero O
			return zero, false
		}
	}
	return v.(O), true
}

func (t *Trie[O]) traverse(targetMap *sync.Map, keys []ComparableOrString) (*sync.Map, any) {
	length := len(keys)
	if length == 0 {
		panic("traverse: empty keys")
	}

	for _, k := range keys[:length-1] {
		v, ok := targetMap.Load(k)
		if !ok {
			newMap := &sync.Map{}
			targetMap.Store(k, newMap)
			v = newMap
		}
		targetMap = v.(*sync.Map)
	}
	return targetMap, keys[length-1]
}

func (t *Trie[O]) Store(keys []ComparableOrString, value O) {
	if swapped := t.size.CompareAndSwap(t.maxSize, 0); swapped {
		t.headIdx = 1 - t.headIdx
	}
	targetMap := t.memos[t.headIdx]
	m, k := t.traverse(targetMap, keys)
	m.Store(k, value)
	t.size.Add(1)
}

func NewTrie[O any](maxSize uint32) Trie[O] {
	if maxSize == 0 {
		panic("maxSize should be greater than 0")
	}
	return Trie[O]{
		memos:   [2]*sync.Map{{}, {}},
		maxSize: maxSize,
	}
}
