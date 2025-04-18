package orderedbuffer_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/on-the-ground/effect_ive_go/effects/internal/orderedbuffer"
)

func TestOrderedBoundedBuffer_InsertAndEviction(t *testing.T) {
	buf := orderedbuffer.NewOrderedBoundedBuffer(3, func(a, b int) int {
		return a - b
	})

	// Insert 5 values, but buffer can only hold 3
	inputs := []int{10, 5, 7, 3, 8} // expected order: 3, 5, 7, 8, 10
	for _, v := range inputs {
		err := buf.Insert(v)
		if err != nil {
			t.Fatalf("unexpected error inserting %d: %v", v, err)
		}
	}

	// Close buffer to flush remaining values
	buf.Close()

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
	buf := orderedbuffer.NewOrderedBoundedBuffer(2, func(a, b int) int {
		return a - b
	})

	_ = buf.Insert(1)
	buf.Close()

	err := buf.Insert(2)
	if !errors.Is(err, orderedbuffer.ErrClosedBuffer) {
		t.Errorf("expected ErrClosedBuffer, got %v", err)
	}
}
