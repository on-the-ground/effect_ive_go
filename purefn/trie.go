package purefn

import (
	"sync"
	"sync/atomic"
)

// The Trie uses a sync.Map to store values, allowing for concurrent access.
// The headIdx is used to switch between two maps when the size exceeds maxSize.
// The size is managed using an atomic counter to ensure thread safety.
type Trie[O any] struct {
	memos   [2]*sync.Map
	headIdx uint32
	size    atomic.Uint32
	maxSize uint32
}

// NewTrie creates a new Trie with the specified maximum size.
func NewTrie[O any](maxSize uint32) Trie[O] {
	if maxSize == 0 {
		panic("maxSize should be greater than 0")
	}
	return Trie[O]{
		memos:   [2]*sync.Map{{}, {}},
		maxSize: maxSize,
	}
}

// Load retrieves a value from the Trie based on the provided keys.
// It first checks the current head map, and if not found, it checks the other map.
// If the value is not found in either map, it returns a zero value and false.
// If the value is found, it returns the value and true.
// The keys must be non-empty and of type ComparableOrString.
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

// traverse navigates through the Trie based on the provided keys.
// It returns the target map and the last key.
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

// Store adds a value to the Trie based on the provided keys.
// If the size exceeds maxSize, it switches to the other map and clears the current one.
// It uses CompareAndSwap to ensure atomicity when checking and updating the size.
func (t *Trie[O]) Store(keys []ComparableOrString, value O) {
	if swapped := t.size.CompareAndSwap(t.maxSize, 0); swapped {
		t.memos[t.headIdx].Clear()
		t.headIdx = 1 - t.headIdx
	}
	targetMap := t.memos[t.headIdx]
	m, k := t.traverse(targetMap, keys)
	m.Store(k, value)
	t.size.Add(1)
}
