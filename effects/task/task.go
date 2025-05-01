package task

import (
	"context"
	"errors"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// WithEffectHandler registers a TaskEffect handler that supports async result retrieval.
func WithEffectHandler[R any](
	ctx context.Context,
	bufferSize int,
) (context.Context, func() context.Context) {
	ctx, endOfTaskHandler := effects.WithResumableEffectHandler(
		ctx,
		bufferSize,
		effectmodel.EffectTask,
		func(ctx context.Context, asyncFn payload[R]) (R, error) {
			done := make(chan handlers.ResumableResult[R], 1)
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
					var zero R
					return zero, errors.New("task result channel closed")
				}
				return res.Value, res.Err
			case <-ctx.Done():
				return *new(R), ctx.Err()
			}
		},
	)

	return ctx, endOfTaskHandler
}

// Effect performs an asynchronous task and returns a channel with the result.
func Effect[R any](ctx context.Context, asyncFn func(context.Context) (R, error)) <-chan handlers.ResumableResult[R] {
	return effects.PerformResumableEffect[payload[R], R](ctx, effectmodel.EffectTask, payload[R](asyncFn))
}

// payload defines an asynchronous operation that returns a value of type R.
type payload[R any] func(context.Context) (R, error)

func (_ payload[R]) PartitionKey() string {
	return "unpartitioned"
}
