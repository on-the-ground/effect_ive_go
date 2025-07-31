package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
	"github.com/on-the-ground/effect_ive_go/shared/orderedbuffer"
	"go.uber.org/zap"
)

const effectKey effects.EffectKey = "github.com/on-the-ground/effect_ive_go/effects/stream/effectKey"

// WithEffectHandler initializes the stream effect handler.
// It allows `Effect(ctx, [...])` to spawn multiple goroutines under managed scope.
func WithEffectHandler[T any](parentCtx context.Context, bufferSize int) (context.Context, func() context.Context) {
	concurrencyHandlerId := concurrency.MustHaveEffectHandler(parentCtx)

	reg := channelRegistry[T]{
		Map: &sync.Map{},
	}
	ctx, endOfStreamHandler := effects.WithResumableEffectHandler(
		parentCtx,
		bufferSize,
		effectKey,
		reg.handleEffect,
	)
	streamHandlerId := MustHaveEffectHandler[T](ctx)

	log.Effect(parentCtx, log.LogInfo, "concurrency handler of this stream handler: ", map[string]interface{}{
		"streamHander":       streamHandlerId,
		"concurrencyHandler": concurrencyHandlerId,
	})

	return ctx, func() context.Context {
		endOfStreamHandler()
		reg.Map.Clear()
		return parentCtx
	}
}

// MapEffect applies a mapping function to each element from the source channel
// and sends the result to the sink channel.
// It closes the sink channel when done.
func MapEffect[T any, R any](
	ctx context.Context,
	source <-chan T,
	sink chan<- R,
	mapFn func(T) R,
) {
	concurrency.Effect(ctx, func(ctx context.Context) {
		defer close(sink)
		for v := range source {
			select {
			case sink <- mapFn(v):
			case <-ctx.Done():
				return
			}
		}
	})
}

// PipeEffect reads from the source channel and writes to the sink channel.
// It closes the sink channel when done.
func PipeEffect[T any](
	ctx context.Context,
	source <-chan T,
	sink chan<- T,
) {
	concurrency.Effect(ctx, func(ctx context.Context) {
		defer close(sink)
		for v := range source {
			select {
			case sink <- v:
			case <-ctx.Done():
				return
			}
		}
	})
}

// EagerFilterEffect filters elements from the source channel based on a predicate
// and sends the matching elements to the sink channel immediately.
// It closes the sink channel when done.
func EagerFilterEffect[T any](
	ctx context.Context,
	source <-chan T,
	sink chan<- T,
	predicate func(T) bool,
) {
	concurrency.Effect(ctx, func(ctx context.Context) {
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
	})
}

// LazyFilterEffect filters elements from the source channel based on a predicate
// and polls the sink channel and checks the predicate on every pollInterval to emulate lazy evaluation.
// It sends the matching elements to the sink channel when the predicate is satisfied at consumption time.
// It closes the sink channel when done.
func LazyFilterEffect[T any](
	ctx context.Context,
	source <-chan T,
	sink chan<- T,
	predicate func(T) bool,
	pollInterval time.Duration,
) {
	concurrency.Effect(ctx, func(ctx context.Context) {
		defer close(sink)

		pollToProduce := func(v T) {
			for {
				if !predicate(v) {
					return
				}
				select {
				case sink <- v:
					return
				case <-ctx.Done():
					return
				default:
				}
				time.Sleep(pollInterval)
			}
		}

		for v := range source {
			pollToProduce(v)
		}

	})
}

// MergeEffect merges multiple source channels into a single sink channel.
// It closes the sink channel when all sources are done.
// It uses a goroutine for each source channel to read from it concurrently.
// The sink channel is closed when all sources are done.
func MergeEffect[T any](
	ctx context.Context,
	sources []<-chan T,
	sink chan<- T,
) {
	localCtx, endOfWorkers := concurrency.WithEffectHandler(ctx, len(sources)*2)

	for _, source := range sources {
		concurrency.Effect(localCtx, func(ctx context.Context) {
			for v := range source {
				select {
				case sink <- v:
				case <-ctx.Done():
					return
				}
			}
		})
	}

	concurrency.Effect(ctx, func(ctx context.Context) {
		endOfWorkers()
		close(sink)
	})
}

// SubscribeEffect subscribes to a source channel and sends the received values to the sink channel.
// It also sends dropped values to the dropped channel.
// It closes the sink channel when done.
// The dropped channel is optional and can be nil.
// If the dropped channel is nil, dropped values are ignored.
// Warning: never close the sink channel before unsubscribing.
// Closing the sink channel will cause a panic if there are still subscribers.
func SubscribeEffect[T any](
	ctx context.Context,
	source SourceAsKey[T],
	sink chan<- T,
	dropped chan<- T,
) {
	effects.PerformResumableEffect[payload[T]](ctx,
		effectKey,
		newSubscribePayload(source, sink, dropped),
	)
}

// UnsubscribeEffect unsubscribes from a source channel.
// It removes the sink channel from the list of subscribers.
func UnsubscribeEffect[T any](
	ctx context.Context,
	source SourceAsKey[T],
	sink chan<- T,
) {
	effects.PerformResumableEffect[payload[T]](ctx,
		effectKey,
		newUnsubscribePayload(source, sink),
	)
}

// OrderByEffect orders the elements from the source channel using a bounded buffer
// and sends the ordered elements to the sink channel.
// It uses an ordered buffer to maintain the order of elements.
// The buffer replaces the temporal window dilimited by the watermark.
// The size of the buffer is the spatial window size for ordering.
func OrderByEffect[T any](
	ctx context.Context,
	windowSize int,
	source SourceAsKey[T],
	sink chan<- T,
	cmpFn orderedbuffer.CompareFunc[T],
) {
	concurrency.Effect(ctx, func(ctx context.Context) {
		buf := orderedbuffer.NewOrderedBoundedBuffer(windowSize, cmpFn)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for ordered := range buf.Source() {
				select {
				case <-ctx.Done():
				case sink <- ordered:
				}
			}
		}()

		defer func() {
			buf.Close(ctx)
			<-done
			close(sink)
		}()

		for v := range source {
			ok := buf.Insert(ctx, v)
			if !ok {
				log.Effect(ctx, log.LogDebug, "ordered buffer closed", map[string]interface{}{})
				return
			}
		}

	})
}

type channelRegistry[T any] struct {
	*sync.Map
}

func (reg channelRegistry[T]) handleEffect(ctx context.Context, msg payload[T]) (any, error) {
	switch msg := msg.(type) {
	case subscribePayload[T]:
		return reg.subscribe(ctx, msg)
	case unsubscribePayload[T]:
		return reg.unsubscribe(ctx, msg)
	default:
		log.Effect(ctx, log.LogError, "unknown message type", map[string]interface{}{
			"msg": msg,
		})
		return struct{}{}, nil
	}
}

func (reg channelRegistry[T]) unsubscribe(ctx context.Context, msg unsubscribePayload[T]) (struct{}, error) {
	oldSinks, ok := helper.GetTypedValueOf2[*RegisteredList[T]](func() (any, bool) {
		return reg.Load(msg.Source.String())
	})
	if !ok {
		log.Effect(ctx, log.LogError, "fail to cast sinks", map[string]interface{}{
			"key": msg.Source,
		})
		return struct{}{}, nil
	}

	getIdxOf := func(oldSinks *RegisteredList[T], targetSink chan<- T) (int, bool) {
		for i, regPair := range oldSinks.Registered {
			if regPair.Sink == targetSink {
				return i, true
			}
		}
		return -1, false
	}
	idx, ok := getIdxOf(oldSinks, msg.Sink)
	if !ok {
		log.Effect(ctx, log.LogInfo, "fail to find sink in sinks", map[string]interface{}{
			"key": msg.Source,
		})
		return struct{}{}, nil
	}
	newSinks := append(oldSinks.Registered[:idx], oldSinks.Registered[idx+1:]...)

	tryUnregisterSink := func() error {
		if swapped := reg.CompareAndSwap(
			msg.Source.String(),
			oldSinks,
			newSinks,
		); swapped {
			// If the swap was successful, we can break out of the loop
			return nil
		}
		// race condition, the sink was already updated
		// We need to retry the operation
		err := fmt.Errorf("fail to update sinks")
		log.Effect(ctx, log.LogDebug, "tryUnregistreSink: ", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
		return err
	}

	maxAttemps := 5
	err := helper.Retry(maxAttemps, tryUnregisterSink)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to append new sink after max attempts", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
	}
	return struct{}{}, err
}

func (reg channelRegistry[T]) subscribe(ctx context.Context, msg subscribePayload[T]) (struct{}, error) {
	var firstSink bool
	newPair := newSinkDropPair(msg.Sink, msg.Dropped)

	raw, ok := reg.Load(msg.Source.String())
	firstSink = !ok
	if firstSink {
		reg.Store(msg.Source.String(), &RegisteredList[T]{Registered: []*sinkDropPair[T]{newPair}})
		concurrency.Effect(ctx, func(ctx context.Context) {
			logger, _ := zap.NewProduction()
			ctx, endOfLogHandler := log.WithZapEffectHandler(ctx, 10, logger)
			defer endOfLogHandler()
			defer func() {
				if r := recover(); r != nil {
					log.Effect(ctx, log.LogError, "panic while registering sink", map[string]interface{}{
						"error": r,
						"key":   msg.Source,
					})
				}
			}()
			reg.arbit(ctx, msg.Source)
		})
		return struct{}{}, nil
	}

	oldSinks, ok := raw.(*RegisteredList[T])
	if !ok {
		log.Effect(ctx, log.LogError, "fail to cast sinks", map[string]interface{}{
			"key": msg.Source,
		})
		return struct{}{}, nil
	}

	tryRegisterSink := func() error {

		if swapped := reg.CompareAndSwap(
			msg.Source.String(),
			oldSinks,
			&RegisteredList[T]{Registered: append(oldSinks.Registered, newPair)},
		); swapped {
			// If the swap was successful, we can break out of the loop
			return nil
		}

		// race condition, the sink was already updated
		// We need to retry the operation
		err := fmt.Errorf("fail to append new sink")
		log.Effect(ctx, log.LogDebug, "tryRegistreSink: ", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
		return err
	}

	maxAttemps := 5
	err := helper.Retry(maxAttemps, tryRegisterSink)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to append new sink after max attempts", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
	}
	return struct{}{}, err
}

func (reg *channelRegistry[T]) arbit(ctx context.Context, source SourceAsKey[T]) {
	var sinks *RegisteredList[T]

	for v := range source {
		var ok bool

		// Intended to reload the sinks when the message is received
		if sinks, ok = helper.GetTypedValueOf2[*RegisteredList[T]](func() (any, bool) {
			return reg.Load(source.String())
		}); !ok {
			log.Effect(ctx, log.LogError, "fail to cast sinks, dropped an message", map[string]interface{}{
				"value": v,
			})
			continue
		}

		if len(sinks.Registered) == 0 {
			continue
		}

		// Send the message to all sinks
		for _, pair := range sinks.Registered {
			sink := pair.Sink
			dropped := pair.Dropped
			if dropped == nil {
				select {
				case sink <- v:
				case <-ctx.Done():
					return
				default:
					log.Effect(ctx, log.LogError, "message dropped:", map[string]interface{}{
						"dropped": v,
					})
				}
			} else {
				select {
				case sink <- v:
				case dropped <- v:
				case <-ctx.Done():
					return
				default:
					log.Effect(ctx, log.LogError, "message dropped:", map[string]interface{}{
						"dropped": v,
					})
				}
			}
		}
	}

	if sinks != nil {
		// Remove the sinks from the registry
		if deleted := reg.CompareAndDelete(source.String(), sinks); deleted {
			for _, chanPair := range sinks.Registered {
				// Close the sink channel
				close(chanPair.Sink)
				close(chanPair.Dropped)
			}
		} else {
			log.Effect(ctx, log.LogError, "fail to unregister sinks", map[string]interface{}{
				"key": source,
			})
		}

	}
}

// MustHaveEffectHandler returns the effect handler ID for the stream effect handler.
// It panics if the effect handler is not found in the context.
func MustHaveEffectHandler[T any](ctx context.Context) string {
	return effects.ResumableEffectHandlerId[payload[T]](ctx, effectKey)
}
