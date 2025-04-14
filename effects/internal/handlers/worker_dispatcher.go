package handlers

import (
	"context"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// --- common interface ---

type WorkerDispatcher[T any] interface {
	GetChannelOf(msg T) chan T
}

// --- single queue ---

type singleQueue[T any] struct {
	effectCh chan T
}

func (q singleQueue[T]) GetChannelOf(_ T) chan T {
	return q.effectCh
}

func NewSingleQueue[T any](
	ctx context.Context,
	bufferSize int,
	handleFn func(context.Context, T),
) WorkerDispatcher[T] {
	effCh := make(chan T, bufferSize)
	ready := make(chan struct{})

	go func(ch chan T) {
		defer close(ch)
		close(ready)
		for {
			select {
			case msg := <-ch:
				handleFn(ctx, msg)
			case <-ctx.Done():
				return
			}
		}
	}(effCh)

	<-ready

	return singleQueue[T]{effectCh: effCh}
}

// --- partitioned queue ---

type partitionedQueue[T effectmodel.Partitionable] struct {
	effectChs []chan T
}

func (pq partitionedQueue[T]) GetChannelOf(msg T) chan T {
	idx := getIndexByHash(msg, len(pq.effectChs))
	return pq.effectChs[idx]
}

func NewPartitionedQueue[T effectmodel.Partitionable](
	ctx context.Context,
	numWorkers, bufferSize int,
	handleFn func(context.Context, T),
) WorkerDispatcher[T] {
	channels := make([]chan T, numWorkers)
	ready := sync.WaitGroup{}
	for i := 0; i < numWorkers; i++ {
		ready.Add(1)
		ch := make(chan T, bufferSize)
		go func(ch chan T) {
			defer close(ch)
			ready.Done()
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
	ready.Wait()
	return partitionedQueue[T]{effectChs: channels}
}
