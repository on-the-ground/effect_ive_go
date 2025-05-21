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

func TestTrie_SwitchingClearsPrevious(t *testing.T) {
	trie := purefn.NewTrie[string](2)

	// 첫 번째 값 저장
	trie.Store([]purefn.ComparableOrString{"x", "1"}, "first")
	trie.Store([]purefn.ComparableOrString{"x", "2"}, "second") // maxSize 도달 → head switch 예정

	// 새 head에서 저장 시작
	trie.Store([]purefn.ComparableOrString{"y", "1"}, "new")

	// 첫 번째 값은 사라졌어야 함 (head switch 후 이전 map은 clear 되었으므로)
	_, ok := trie.Load([]purefn.ComparableOrString{"x", "1"})
	assert.False(t, ok, "Expected first inserted value to be evicted after head switch")

	// 새 값은 여전히 존재해야 함
	val, ok := trie.Load([]purefn.ComparableOrString{"y", "1"})
	assert.True(t, ok)
	assert.Equal(t, "new", val)
}
