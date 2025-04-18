package effects

import (
	"context"
)

type streamEffectPayload interface {
	sealedStreamEffectPayload()
}

type MapStreamPayload[T any, R any] struct {
	Source <-chan T
	Sink   chan<- R
	MapFn  func(T) R
}

func (m MapStreamPayload[T, R]) sealedStreamEffectPayload() {}

type FilterStreamPayload[T any] struct {
	Source    <-chan T
	Sink      chan<- T
	Predicate func(T) bool
}

func (f FilterStreamPayload[T]) sealedStreamEffectPayload() {}

type MergeStreamPayload[T any] struct {
	Sources []<-chan T
	Sink    chan<- T
}

func (p MergeStreamPayload[T]) sealedStreamEffectPayload() {}

func WithStreamEffectHandler(ctx context.Context, bufferSize int) (context.Context, func() context.Context) {
	return WithConcurrencyEffectHandler(ctx, bufferSize)
}

func StreamEffect[T any, R any](ctx context.Context, payload streamEffectPayload) {
	switch msg := payload.(type) {
	case MapStreamPayload[T, R]:
		ConcurrencyEffect(ctx, func(ctx context.Context) {
			mapFn(ctx, msg.Source, msg.Sink, msg.MapFn)
		})
	case FilterStreamPayload[T]:
		ConcurrencyEffect(ctx, func(ctx context.Context) {
			filterFn(ctx, msg.Source, msg.Sink, msg.Predicate)
		})
	case MergeStreamPayload[T]:
		for _, source := range msg.Sources {
			ConcurrencyEffect(ctx, func(ctx context.Context) {
				pipeFn(ctx, source, msg.Sink)
			})
		}
	default:
	}
}

func mapFn[T any, R any](ctx context.Context, source <-chan T, sink chan<- R, f func(T) R) {
	defer close(sink)
	for v := range source {
		select {
		case sink <- f(v):
		case <-ctx.Done():
			return
		}
	}
}

func pipeFn[T any](ctx context.Context, source <-chan T, sink chan<- T) {
	mapFn(ctx, source, sink, func(v T) T {
		return v
	})
}

func filterFn[T any](ctx context.Context, source <-chan T, sink chan<- T, predicate func(T) bool) {
	defer close(sink)
	for v := range source {
		if predicate(v) {
			select {
			case sink <- v:
			case <-ctx.Done():
				return
			}
		}
	}
}
