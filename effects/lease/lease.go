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

func EffectResourceRegistration(ctx context.Context, key string, numOwners int) (bool, error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return state.Effect(ctx, state.InsertPayloadOf(
			key,
			make(chan struct{}, numOwners),
		))
	})
}

func EffectResourceDeregistration(ctx context.Context, key string) (bool, error) {
	if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
		return state.Effect(ctx, state.LoadPayloadOf(key))
	}); err != nil {
		return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
	} else if len(ch) != 0 {
		return false, fmt.Errorf("%w key %s", ErrResourceInUse, key)
	} else {
		return helper.GetTypedValueOf[bool](func() (any, error) {
			return state.Effect(ctx, state.CADPayloadOf(key, ch))
		})
	}
}

func EffectAcquisition(ctx context.Context, key string) (bool, error) {
	if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
		return state.Effect(ctx, state.LoadPayloadOf(key))
	}); err != nil {
		return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
	} else {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case ch <- struct{}{}:
			return true, nil
		}
	}
}

func EffectRelease(ctx context.Context, key string) (bool, error) {
	if ch, err := helper.GetTypedValueOf[chan struct{}](func() (any, error) {
		return state.Effect(ctx, state.LoadPayloadOf(key))
	}); err != nil {
		return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
	} else {
		select {
		case <-ch:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}
