package orderedbuffer

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
)

var ErrClosedBuffer = errors.New("buffer is closed")

type CompareFunc[T any] func(a, b T) int

type OrderedBoundedBuffer[T any] struct {
	mu sync.Mutex

	data      []T
	maxBufLen int
	compare   CompareFunc[T]

	sink   chan T
	closed atomic.Bool
}

func NewOrderedBoundedBuffer[T any](maxBufLen int, cmp CompareFunc[T]) *OrderedBoundedBuffer[T] {
	return &OrderedBoundedBuffer[T]{
		data:      make([]T, 0, maxBufLen),
		maxBufLen: maxBufLen,
		compare:   cmp,
		sink:      make(chan T, maxBufLen*2),
	}
}

func (b *OrderedBoundedBuffer[T]) Insert(ctx context.Context, val T) bool {
	if b.closed.Load() {
		return false
	}

	b.mu.Lock()

	idx := sort.Search(len(b.data), func(i int) bool {
		return b.compare(val, b.data[i]) < 0
	})

	b.data = append(b.data, val)
	copy(b.data[idx+1:], b.data[idx:])
	b.data[idx] = val

	var evictedVal T
	evicted := false
	if len(b.data) > b.maxBufLen {
		evictedVal = b.data[0]
		b.data = b.data[1:]
		evicted = true
	}

	b.mu.Unlock()

	if evicted {
		select {
		case <-ctx.Done():
		case b.sink <- evictedVal:
		}
	}

	return true
}

func (b *OrderedBoundedBuffer[T]) Source() <-chan T {
	return b.sink
}

func (b *OrderedBoundedBuffer[T]) Close(ctx context.Context) {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		for _, v := range b.data {
			select {
			case <-ctx.Done():
				return
			case b.sink <- v:
			}
		}
		close(b.sink)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
