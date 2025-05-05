package lease

import (
	"context"
	"errors"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

/* func WithDistributedEffectHandler() {
	return state.WithEffectHandler()
} */

func WithInMemoryEffectHandler(ctx context.Context, bufferSize, numWorkers int) (context.Context, func() context.Context) {
	return state.WithEffectHandler[string, chan struct{}](
		ctx,
		bufferSize, numWorkers,
		false,
		state.NewInMemoryStore[string](),
		nil,
	)
}

var ErrUnregisteredResource = errors.New("unregistered resource")
var ErrResourceInUse = errors.New("unable to deregister resource in use")

func Effect(ctx context.Context, payload payload) (bool, error) {
	switch payload := payload.(type) {

	case register:
		return helper.GetTypedValueOf[bool](func() (any, error) {
			return state.Effect(ctx, state.InsertPayloadOf(
				payload.Key,
				make(chan struct{}, payload.NumOwners),
			))
		})

	case deregister:
		if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
			return state.Effect(ctx, state.LoadPayloadOf(payload.Key))
		}); err != nil {
			return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, payload.Key)
		} else if len(ch) != 0 {
			return false, fmt.Errorf("%w key %s", ErrResourceInUse, payload.Key)
		} else {
			return helper.GetTypedValueOf[bool](func() (any, error) {
				return state.Effect(ctx, state.CADPayloadOf(
					payload.Key,
					ch,
				))
			})
		}

	case acquire:
		if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
			return state.Effect(ctx, state.LoadPayloadOf(payload.Key))
		}); err != nil {
			return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, payload.Key)
		} else {
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case ch <- struct{}{}:
				return true, nil
			}
		}

	case release:
		if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
			return state.Effect(ctx, state.LoadPayloadOf(payload.Key))
		}); err != nil {
			return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, payload.Key)
		} else {
			select {
			case <-ch:
				return true, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}

	default:
		panic("exhaustive match")
	}
}
