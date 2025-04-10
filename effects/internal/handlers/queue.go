package handlers

import (
	"context"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// --- common interface ---

type queue[T any] interface {
	getChannelOf(msg T) chan T
}

// --- single queue ---

var _ queue[any] = singleQueue[any]{}

type singleQueue[T any] struct {
	effectCh chan T
}

func (q singleQueue[T]) getChannelOf(_ T) chan T {
	return q.effectCh
}

func newSingleQueue[T any](
	ctx context.Context,
	bufferSize int,
	handleFn func(context.Context, T),
) singleQueue[T] {
	effCh := make(chan T, bufferSize)
	go func(ch chan T) {
		defer close(ch)
		for {
			select {
			case msg := <-ch:
				handleFn(ctx, msg)
			case <-ctx.Done():
				return
			}
		}
	}(effCh)
	return singleQueue[T]{effectCh: effCh}
}

// --- partitioned queue ---

var _ queue[effectmodel.Partitionable] = partitionedQueue[effectmodel.Partitionable]{}

type partitionedQueue[T effectmodel.Partitionable] struct {
	effectChs []chan T
}

func (pq partitionedQueue[T]) getChannelOf(msg T) chan T {
	idx := getIndexByHash(msg, len(pq.effectChs))
	return pq.effectChs[idx]
}

func newPartitionedQueue[T effectmodel.Partitionable](
	ctx context.Context,
	numWorkers, bufferSize int,
	handleFn func(context.Context, T),
) partitionedQueue[T] {
	channels := make([]chan T, numWorkers)
	for i := 0; i < numWorkers; i++ {
		ch := make(chan T, bufferSize)
		go func(ch chan T) {
			defer close(ch)
			for {
				select {
				case msg := <-ch:
					handleFn(ctx, msg)
				case <-ctx.Done():
					return
				}
			}
		}(ch)
		channels[i] = ch
	}
	return partitionedQueue[T]{effectChs: channels}
}
