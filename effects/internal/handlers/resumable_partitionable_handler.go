package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

func NewResumablePartitionableEffectHandler[T effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T) R,
	teardown func(),
) ResumablePartitionableHandler[T, R] {
	return ResumablePartitionableHandler[T, R]{
		resumablePartitionableEffectScope: newResumablePartitionableEffectScope(ctx, config, handleFn, teardown),
	}
}

type ResumablePartitionableHandler[T effectmodel.Partitionable, R any] struct {
	*resumablePartitionableEffectScope[T, R]
}

func (rh ResumablePartitionableHandler[T, R]) PerformEffect(ctx context.Context, payload T) (ret R) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf(
				"panic while sending to closed channel for effect: %+v",
				map[string]interface{}{
					"effectId": rh.EffectId,
					"payload":  payload,
				},
			)
		}
	}()

	resumeCh := make(chan R, 1)

	msg := resumablePartitionableEffectMessage[T, R]{
		payload:  payload,
		resumeCh: resumeCh,
	}
	select {
	case <-ctx.Done():
	case rh.effectChs[getIndexByHash(msg.payload, len(rh.effectChs))] <- msg:
	}

	select {
	case ret = <-resumeCh:
	case <-ctx.Done():
	}
	return
}

// IMPORTANT:
// This effect handler is **intentionally NOT thread-safe**.
//
// It is designed with the assumption that each handler instance will be used
// only within a **single goroutine** and **single execution scope**.
//
// ➤ We deliberately avoided synchronization (mutexes, atomic ops, etc.)
//
//	to keep the handler lightweight and avoid accidental cross-goroutine sharing.
//
// ➤ Sharing this handler or its context across multiple goroutines
//
//	will lead to **undefined behavior**, including data races, panics, or deadlocks.
//
// This is a **conscious design choice** to reinforce proper scoping and ownership.
// If you require shared access, explicitly manage synchronization outside this scope.
type resumablePartitionableEffectScope[T effectmodel.Partitionable, R any] struct {
	EffectId  string
	effectChs []chan resumablePartitionableEffectMessage[T, R]
	closeFn   func()
	closed    bool
}

func (es *resumablePartitionableEffectScope[T, R]) Close() {
	if !es.closed {
		es.closeFn()
		es.closed = true
		log.Printf("Effect scope closed: %s", es.EffectId)
	}
}

func newResumablePartitionableEffectScope[T effectmodel.Partitionable, R any](
	ctx context.Context,
	config effectmodel.EffectScopeConfig,
	handleFn func(context.Context, T) R,
	teardown func(),
) *resumablePartitionableEffectScope[T, R] {
	ctx, cancelFn := context.WithCancel(ctx)
	effChs := make([]chan resumablePartitionableEffectMessage[T, R], config.NumWorkers)
	for i := 0; i < config.NumWorkers; i++ {
		effCh := make(chan resumablePartitionableEffectMessage[T, R], config.BufferSize)
		go func(ch chan resumablePartitionableEffectMessage[T, R]) {
			defer close(ch)
			for {
				select {
				case msg := <-ch:
					msg.resumeCh <- handleFn(ctx, msg.payload)
				case <-ctx.Done():
					return
				}
			}
		}(effCh)
		effChs[i] = effCh
	}

	return &resumablePartitionableEffectScope[T, R]{
		EffectId:  uuid.New().String(),
		effectChs: effChs,
		closeFn: func() {
			cancelFn()
			teardown()
		},
		closed: false,
	}
}

type resumablePartitionableEffectMessage[T effectmodel.Partitionable, R any] struct {
	payload  T
	resumeCh chan R
}
