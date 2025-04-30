package orderedbuffer_test

import (
	"context"
	"slices"
	"testing"

	"github.com/on-the-ground/effect_ive_go/shared/orderedbuffer"
	"github.com/stretchr/testify/assert"
)

func TestOrderedBoundedBuffer_InsertAndEviction(t *testing.T) {
	ctx := context.Background()
	buf := orderedbuffer.NewOrderedBoundedBuffer(3, func(a, b int) int {
		return a - b
	})

	// Insert 5 values, but buffer can only hold 3
	inputs := []int{10, 5, 7, 3, 8} // expected order: 3, 5, 7, 8, 10
	for _, v := range inputs {
		ok := buf.Insert(ctx, v)
		assert.Truef(t, ok, "unexpected error inserting %d", v)
	}

	// Close buffer to flush remaining values
	buf.Close(ctx)

	// Drain from Source
	var got []int
	for v := range buf.Source() {
		got = append(got, v)
	}

	// expected results: evicted 3, 5 → flushed 7, 8, 10 → 총 [3 5 7 8 10]
	want := []int{3, 5, 7, 8, 10}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestOrderedBoundedBuffer_InsertAfterClose(t *testing.T) {
	ctx := context.Background()

	buf := orderedbuffer.NewOrderedBoundedBuffer(2, func(a, b int) int {
		return a - b
	})

	_ = buf.Insert(ctx, 1)
	buf.Close(ctx)

	ok := buf.Insert(ctx, 2)
	assert.False(t, ok, "expected ErrClosedBuffer")
}
