package state

import (
	"context"
	"sync"

	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
)

type inMemStore[K comparable] struct {
	*sync.Map
}

func (t inMemStore[K]) CompareAndDelete(k K, old any) (ok bool, err error) {
	ok = t.Map.CompareAndDelete(k, old)
	return
}

func (t inMemStore[K]) CompareAndSwap(k K, old, new any) (ok bool, err error) {
	ok = t.Map.CompareAndSwap(k, old, new)
	return
}

func (t inMemStore[K]) Load(k K) (v any, ok bool, err error) {
	v, ok = t.Map.Load(k)
	return
}

func (t inMemStore[K]) InsertIfAbsent(k K, v any) (ok bool, err error) {
	_, loaded := t.Map.LoadOrStore(k, v)
	return !loaded, nil
}

func NewInMemoryStore[K comparable]() StateStore {
	return NewCasStore(inMemStore[K]{Map: &sync.Map{}})
}

// EffectSubscribeSource subscribes to the effect source and sends the received payloads to the provided sink and dropped channels.
// It logs an error if the effect source is not found.
// WARNING: Stream effect handler must be created before calling this function.
func EffectSubscribeSource(
	ctx context.Context,
	sink chan<- TimeBoundedPayload,
	dropped chan<- TimeBoundedPayload,
) {
	src, err := effectSource(ctx)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to subscribe source, effect source not found", map[string]interface{}{
			"err": err,
		})
		return
	}
	stream.EffectSubscribe(ctx, src, sink, dropped)
}
