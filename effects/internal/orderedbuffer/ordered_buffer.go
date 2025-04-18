package orderedbuffer

import (
	"errors"
	"sort"
)

var ErrClosedBuffer = errors.New("buffer is closed")

type CompareFunc[T any] func(a, b T) int

type OrderedBoundedBuffer[T any] struct {
	data     []T
	capacity int
	compare  CompareFunc[T]

	sink   chan T
	closed bool
}

func NewOrderedBoundedBuffer[T any](capacity int, cmp CompareFunc[T]) *OrderedBoundedBuffer[T] {
	return &OrderedBoundedBuffer[T]{
		data:     make([]T, 0, capacity),
		capacity: capacity,
		compare:  cmp,
		sink:     make(chan T, capacity*2), // 여유 버퍼
	}
}

func (b *OrderedBoundedBuffer[T]) Insert(val T) error {
	if b.closed {
		return ErrClosedBuffer
	}

	idx := sort.Search(len(b.data), func(i int) bool {
		return b.compare(val, b.data[i]) < 0
	})

	b.data = append(b.data, val)
	copy(b.data[idx+1:], b.data[idx:])
	b.data[idx] = val

	if len(b.data) > b.capacity {
		evicted := b.data[0]
		b.data = b.data[1:]
		b.sink <- evicted
	}

	return nil
}

func (b *OrderedBoundedBuffer[T]) Source() <-chan T {
	return b.sink
}

func (b *OrderedBoundedBuffer[T]) Close() {
	if b.closed {
		return
	}
	b.closed = true

	done := make(chan struct{})
	go func() {
		for _, v := range b.data {
			b.sink <- v
		}
		close(b.sink)
		close(done)
	}()
	<-done
}
