package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
	"github.com/on-the-ground/effect_ive_go/shared/orderedbuffer"
	"go.uber.org/zap"
)

func WithEffectHandler[T any](parentCtx context.Context, bufferSize int) (context.Context, func() context.Context) {

	ctx, endOfConcurrencyHandler := concurrency.WithEffectHandler(parentCtx, bufferSize)

	reg := channelRegistry[T]{
		Map: &sync.Map{},
	}
	ctx, endOfStreamHandler := effects.WithFireAndForgetEffectHandler(
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

func EffectMap[T any, R any](
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

func EffectEagerFilter[T any](
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

func EffectLazyFilter[T any](
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

func EffectMerge[T any](
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

func EffectSubscribe[T any](
	ctx context.Context,
	Source SourceAsKey[T],
	Target *sinkDropPair[T],
) {
	effects.FireAndForgetEffect(ctx, effectmodel.EffectStream, subscribePayload[T]{
		Source: Source,
		Target: Target,
	})
}

func EffectOrderBy[T any](
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

func (reg channelRegistry[T]) handleSubscriptionEffect(ctx context.Context, msg subscribePayload[T]) {
	var firstSink bool

	raw, ok := reg.Load(msg.Source.String())
	firstSink = !ok
	if firstSink {
		reg.Store(msg.Source.String(), &RegisteredList[T]{Registered: []*sinkDropPair[T]{msg.Target}})
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
		return
	}

	oldSinks, ok := raw.(*RegisteredList[T])
	if !ok {
		log.Effect(ctx, log.LogError, "fail to cast sinks", map[string]interface{}{
			"key": msg.Source,
		})
		return
	}

	tryRegisterSink := func() error {

		if swapped := reg.CompareAndSwap(
			msg.Source.String(),
			oldSinks,
			&RegisteredList[T]{Registered: append(oldSinks.Registered, msg.Target)},
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
	if err := helper.Retry(maxAttemps, tryRegisterSink); err != nil {
		log.Effect(ctx, log.LogError, "fail to append new sink after max attempts", map[string]interface{}{
			"error": err,
			"key":   msg.Source,
		})
		return
	}

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
