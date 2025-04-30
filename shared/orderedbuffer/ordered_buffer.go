package orderedbuffer

import (
	"context"
	"errors"
	"sort"
	"sync/atomic"
)

var ErrClosedBuffer = errors.New("buffer is closed")

type CompareFunc[T any] func(a, b T) int

type OrderedBoundedBuffer[T any] struct {
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

	// 이진 탐색 후 삽입
	idx := sort.Search(len(b.data), func(i int) bool {
		return b.compare(val, b.data[i]) < 0
	})

	b.data = append(b.data, val)
	copy(b.data[idx+1:], b.data[idx:])
	b.data[idx] = val

	// eviction 발생 시 가장 앞 항목을 sink로 보냄
	if len(b.data) > b.maxBufLen {
		evicted := b.data[0]
		b.data = b.data[1:]
		select {
		case <-ctx.Done():
			return false
		case b.sink <- evicted:
		}
	}

	return true
}

func (b *OrderedBoundedBuffer[T]) Source() <-chan T {
	return b.sink
}

func (b *OrderedBoundedBuffer[T]) Close(ctx context.Context) {
	// 이미 닫혀 있으면 중복 호출 무시
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
