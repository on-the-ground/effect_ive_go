// Package lease provides a lightweight lease/semaphore mechanism
// using the state effect system as backend storage.
package lease

import (
	"context"
	"errors"
	"fmt"

	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

var (
	// ErrUnregisteredResource is returned when the lease key is not registered in the state.
	ErrUnregisteredResource = errors.New("unregistered resource")
	// ErrResourceInUse is returned when a deregistration is attempted on an in-use lease.
	ErrResourceInUse = errors.New("unable to deregister resource in use")
)

// WithDistributedEffectHandler returns a context with an distributed lease handler using the state effect system as backend storage.
func WithDistributedEffectHandler() {
	//return state.WithEffectHandler()
}

// WithInMemoryEffectHandler returns a context with an in-memory lease handler using buffered channels.
// Each key will map to a `chan struct{}` of size `numOwners`, acting like a semaphore.
func WithInMemoryEffectHandler(ctx context.Context, bufferSize, numWorkers int) (context.Context, func() context.Context) {
	return state.WithEffectHandler[string, chan struct{}](
		ctx,
		bufferSize, numWorkers,
		false,
		state.NewInMemoryStore[string](),
		nil,
	)
}

// EffectResourceRegistration registers a lease resource (key) with a max number of concurrent owners.
// Internally stores a buffered channel of size `numOwners` as the semaphore.
func EffectResourceRegistration(ctx context.Context, key string, numOwners int) (bool, error) {
	return helper.GetTypedValueOf[bool](func() (any, error) {
		return state.Effect(ctx, state.InsertPayloadOf(
			key,
			make(chan struct{}, numOwners),
		))
	})
}

// EffectResourceDeregistration attempts to remove the lease resource (key) from state.
// Deregistration fails if the resource is currently acquired (non-empty channel).
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

// EffectAcquisition attempts to acquire a lease for the given key.
// If the resource is registered and capacity is available, the lease is granted.
// If capacity is full, this call blocks unless context expires.
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

// EffectRelease releases a previously acquired lease for the given key.
// If the lease was not acquired or key is missing, it returns an error.
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
