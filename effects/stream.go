package effects

import (
	"context"
	"fmt"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"go.uber.org/zap"
)

type streamEffectPayload interface {
	sealedStreamEffectPayload()
}

type MapStreamPayloadAny interface {
	Run(ctx context.Context)
}

type MapStreamPayload[T any, R any] struct {
	Source <-chan T
	Sink   chan<- R
	MapFn  func(T) R
}

func (m MapStreamPayload[T, R]) Run(ctx context.Context) {
	mapFn(ctx, m.Source, m.Sink, m.MapFn)
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

type SubscribeStreamPayload[T any] struct {
	Source SourceAsKey[T]
	Sink   chan<- T
}

func (p SubscribeStreamPayload[T]) sealedStreamEffectPayload() {}
func (p SubscribeStreamPayload[T]) PartitionKey() string {
	return p.Source.String()
}

func WithStreamEffectHandler[T any](parentCtx context.Context, bufferSize int) (context.Context, func() context.Context) {

	ctx, endOfConcurrencyHandler := WithConcurrencyEffectHandler(parentCtx, bufferSize)

	reg := channelRegistry[T]{
		Map: &sync.Map{},
	}
	ctx, endOfStreamHandler := WithFireAndForgetEffectHandler(
		ctx,
		bufferSize,
		effectmodel.EffectStream,
		reg.handleSubscriptionEffect,
	)

	return ctx, func() context.Context {
		endOfStreamHandler()
		reg.Map.Clear()
		endOfConcurrencyHandler()
		return parentCtx
	}
}

type SourceAsKey[T any] <-chan T

func (s SourceAsKey[T]) String() string {
	return fmt.Sprintf("%p", s)
}

type SinkList[T any] struct {
	List []chan<- T
}

func StreamEffect[T any](ctx context.Context, payload streamEffectPayload) {
	switch msg := payload.(type) {
	case MapStreamPayloadAny:
		ConcurrencyEffect(ctx, msg.Run)
	case FilterStreamPayload[T]:
		ConcurrencyEffect(ctx, func(ctx context.Context) {
			filter(ctx, msg.Source, msg.Sink, msg.Predicate)
		})
	case MergeStreamPayload[T]:
		for _, source := range msg.Sources {
			ConcurrencyEffect(ctx, func(ctx context.Context) {
				pipe(ctx, source, msg.Sink)
			})
		}
	case SubscribeStreamPayload[T]:
		FireAndForgetEffect(ctx, effectmodel.EffectStream, msg)

	default:
		// StreamEffect is sealed interface, so this should never happen
		// Bug in the code
		panic(fmt.Sprintf("unrecognized stream effect payload: %T", msg))
	}
}

type channelRegistry[T any] struct {
	*sync.Map
}

func (reg channelRegistry[T]) handleSubscriptionEffect(ctx context.Context, msg SubscribeStreamPayload[T]) {
	var firstSink bool

	raw, ok := reg.Load(msg.Source.String())
	firstSink = !ok
	if firstSink {
		reg.Store(msg.Source.String(), &SinkList[T]{List: []chan<- T{msg.Sink}})
		ConcurrencyEffect(ctx, func(ctx context.Context) {
			logger, _ := zap.NewProduction()
			ctx, endOfLogHandler := WithZapLogEffectHandler(ctx, 10, logger)
			defer endOfLogHandler()
			defer func() {
				if r := recover(); r != nil {
					LogEffect(ctx, LogError, "panic while registering sink", map[string]interface{}{
						"error": r,
						"key":   msg.Source,
					})
				}
			}()
			reg.arbit(ctx, msg.Source)
		})
		return
	}

	oldSinks, ok := raw.(*SinkList[T])
	if !ok {
		LogEffect(ctx, LogError, "fail to cast sinks", map[string]interface{}{
			"key": msg.Source,
		})
		return
	}

	tryRegisterSink := func() error {

		if swapped := reg.CompareAndSwap(
			msg.Source.String(),
			oldSinks,
			&SinkList[T]{List: append(oldSinks.List, msg.Sink)},
		); swapped {
			// If the swap was successful, we can break out of the loop
			return nil
		}

		// race condition, the sink was already updated
		// We need to retry the operation
		err := fmt.Errorf("fail to append new sink")
		LogEffect(ctx, LogDebug, "tryRegistreSink: ", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
		return err
	}

	maxAttemps := 5
	if err := retry(maxAttemps, tryRegisterSink); err != nil {
		LogEffect(ctx, LogError, "fail to append new sink after max attempts", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
		return
	}

}

func (reg *channelRegistry[T]) arbit(ctx context.Context, source SourceAsKey[T]) {
	var sinks *SinkList[T]
	defer func() {
		if sinks == nil {
			LogEffect(ctx, LogError, "sinks is nil; skipping cleanup", map[string]interface{}{"key": source})
			return
		}
		for _, sink := range sinks.List {
			// Close the sink channel
			close(sink)
		}
		// Remove the sinks from the registry
		if deleted := reg.CompareAndDelete(source.String(), sinks); !deleted {
			LogEffect(ctx, LogError, "fail to unregister sinks", map[string]interface{}{
				"key": source,
			})
		}
	}()
	for v := range source {
		// Reload the sinks when the message is received
		if raw, ok := reg.Load(source.String()); !ok {
			LogEffect(ctx, LogError, "fail to load sinks, dropped an message", map[string]interface{}{
				"value": v,
			})
		} else if sinks, ok = raw.(*SinkList[T]); !ok {
			LogEffect(ctx, LogError, "fail to cast sinks, dropped an message", map[string]interface{}{
				"value": v,
			})
		}

		// Send the message to all sinks
		for _, sink := range sinks.List {
			select {
			case sink <- v:
			case <-ctx.Done():
				return
			default:
				// If the sink is full, we can drop the message
				LogEffect(ctx, LogError, "fail to send message to sink, dropped an message", map[string]interface{}{
					"value": v,
				})
			}
		}
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

func pipe[T any](ctx context.Context, source <-chan T, sink chan<- T) {
	mapFn(ctx, source, sink, func(v T) T {
		return v
	})
}

func filter[T any](ctx context.Context, source <-chan T, sink chan<- T, predicate func(T) bool) {
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
