package task

import (
	"context"
	"errors"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
)

const effectKey effects.EffectKey = "github.com/on-the-ground/effect_ive_go/effects/task/effectKey"

// WithEffectHandler registers a TaskEffect handler that supports async result retrieval.
func WithEffectHandler(
	ctx context.Context,
	bufferSize int,
) (context.Context, func() context.Context) {
	ctx, endOfTaskHandler := effects.WithResumableEffectHandler(
		ctx,
		bufferSize,
		effectKey,
		func(ctx context.Context, asyncFn payload) (any, error) {
			done := make(chan handlers.ResumableResult, 1)
			ready := make(chan struct{})
			go func() {
				close(ready)

				select {
				case <-ctx.Done():
					close(done)
					return
				default:
				}

				res := handlers.ResumableResultFrom(asyncFn(ctx))

				select {
				case <-ctx.Done():
					// don't send result
				default:
					done <- res
				}
				close(done)
			}()
			<-ready

			select {
			case res, ok := <-done:
				if !ok {
					var zero any
					return zero, errors.New("task result channel closed")
				}
				return res.Value, res.Err
			case <-ctx.Done():
				return *new(any), ctx.Err()
			}
		},
	)

	return ctx, endOfTaskHandler
}

// Effect performs an asynchronous task and returns a channel with the result.
func Effect(ctx context.Context, asyncFn func(context.Context) (any, error)) <-chan handlers.ResumableResult {
	return effects.PerformResumableEffect(ctx, effectKey, payload(asyncFn))
}

// payload defines an asynchronous operation that returns a value of type R.
type payload func(context.Context) (any, error)

func (_ payload) PartitionKey() string {
	return "unpartitioned"
}
