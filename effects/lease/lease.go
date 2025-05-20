// Package lease provides a lightweight lease/semaphore mechanism
// using the state effect system as backend storage.
package lease

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

var (
	// ErrUnregisteredResource is returned when the lease key is not registered in the state.
	ErrUnregisteredResource = errors.New("unregistered resource")
	// ErrResourceInUse is returned when a deregistration is attempted on an in-use lease.
	ErrResourceInUse = errors.New("unable to deregister resource in use")
)

// WithDistributedEffectHandler returns a context with an distributed lease handler using the state effect system as backend storage.
// todo func WithDistributedEffectHandler() {}

// WithInMemoryEffectHandler returns a context with an in-memory lease handler using buffered channels.
// Each key will map to a `peekable[time.Time]` of size `numOwners`, acting like a semaphore.
func WithInMemoryEffectHandler(ctx context.Context, bufferSize, numWorkers int) (context.Context, func() context.Context) {
	ctxStream, endOfStreamHandler := stream.WithEffectHandler[time.Time](ctx, bufferSize)
	ctxStreamState, endOfStateHandler := state.WithEffectHandler[string, *sourceSinkPair[time.Time]](
		ctxStream,
		bufferSize, numWorkers,
		false,
		state.NewInMemoryStore[string](),
		nil,
	)
	return ctxStreamState, func() context.Context {
		endOfStateHandler()
		endOfStreamHandler()
		return ctx
	}
}

// ResourceRegistrationEffect registers a lease resource (key) with a max number of concurrent owners.
// Internally stores a buffered channel of size `numOwners` as the semaphore.
func ResourceRegistrationEffect(
	ctx context.Context,
	key string,
	numOwners int,
	ttl, pollInterval time.Duration,
) (bool, error) {
	expire := func(ts time.Time) bool {
		return ts.Add(ttl).After(time.Now())
	}

	peekable := newFilterablePair[time.Time](numOwners)

	stream.LazyFilterEffect(ctx,
		peekable.source,
		peekable.sink,
		expire,
		pollInterval,
	)

	return helper.GetTypedValueOf[bool](func() (any, error) {
		return state.InsertIfAbsentEffect(ctx,
			key,
			peekable,
		)
	})
}

func ResourceRegistrationNoExpiryEffect(
	ctx context.Context,
	key string,
	numOwners int,
) (bool, error) {
	pair := newBypassPair[time.Time](numOwners)

	return helper.GetTypedValueOf[bool](func() (any, error) {
		return state.InsertIfAbsentEffect(ctx,
			key,
			pair,
		)
	})
}

// ResourceDeregistrationEffect attempts to remove the lease resource (key) from state.
// Deregistration fails if the resource is currently acquired (non-empty channel).
func ResourceDeregistrationEffect(ctx context.Context, key string) (res bool, err error) {
	var peekable *sourceSinkPair[time.Time]
	peekable, err = state.LoadEffect[string, *sourceSinkPair[time.Time]](ctx, key)
	if err != nil {
		res, err = false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
		return
	}

	if len(peekable.source) > 0 {
		res, err = false, fmt.Errorf("%w key %s", ErrResourceInUse, key)
		return
	}

	defer func() {
		if res {
			close(peekable.source)
		}
	}()
	res, err = state.CompareAndDeleteEffect(ctx, key, peekable)
	return

}

// AcquisitionEffect attempts to acquire a lease for the given key.
// If the resource is registered and capacity is available, the lease is granted.
// If capacity is full, this call blocks unless context expires.
func AcquisitionEffect(ctx context.Context, key string) (bool, error) {
	if peekable, err := state.LoadEffect[string, *sourceSinkPair[time.Time]](ctx, key); err != nil {
		return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
	} else {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case peekable.source <- time.Now():
			return true, nil
		}
	}
}

// ReleaseEffect releases a previously acquired lease for the given key.
// If the lease was not acquired or key is missing, it returns an error.
func ReleaseEffect(ctx context.Context, key string) (bool, error) {
	if peekable, err := state.LoadEffect[string, *sourceSinkPair[time.Time]](ctx, key); err != nil {
		return false, fmt.Errorf("%w: key %s", ErrUnregisteredResource, key)
	} else {
		select {
		case <-peekable.sink:
			return true, nil
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			return true, nil
		}
	}
}

type sourceSinkPair[T any] struct {
	source, sink chan T
}

func newFilterablePair[T any](numOwners int) *sourceSinkPair[T] {
	return &sourceSinkPair[T]{
		source: make(chan T, numOwners),
		sink:   make(chan T),
	}
}

func newBypassPair[T any](numOwners int) *sourceSinkPair[T] {
	ch := make(chan T, numOwners)
	return &sourceSinkPair[T]{
		source: ch,
		sink:   ch,
	}
}
