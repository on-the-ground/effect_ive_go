package purefn_test

import (
	"testing"

	"github.com/on-the-ground/effect_ive_go/purefn"
	"github.com/stretchr/testify/assert"
)

func TestTrie_BasicUsage(t *testing.T) {
	trie := purefn.NewTrie[string](1)

	// store a value
	trie.Store([]purefn.ComparableOrString{"a", "b", "c"}, "final")

	// load it back
	val, ok := trie.Load([]purefn.ComparableOrString{"a", "b", "c"})
	assert.True(t, ok)
	assert.Equal(t, "final", val)

	// wrong key path
	_, ok = trie.Load([]purefn.ComparableOrString{"a", "b", "x"})
	assert.False(t, ok)

	// overwrite existing
	trie.Store([]purefn.ComparableOrString{"a", "b", "c"}, "updated")
	val, ok = trie.Load([]purefn.ComparableOrString{"a", "b", "c"})
	assert.True(t, ok)
	assert.Equal(t, "updated", val)
}

func TestTrie_EmptyKeysPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on empty keys, but didn't panic")
		}
	}()
	trie := purefn.NewTrie[int](2)
	trie.Load([]purefn.ComparableOrString{})
}
