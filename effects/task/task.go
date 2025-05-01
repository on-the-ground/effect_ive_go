package task

import (
	"context"
	"errors"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// TaskPayload defines an asynchronous operation that returns a value of type R.
type TaskPayload[R any] func(context.Context) (R, error)

func (_ TaskPayload[R]) PartitionKey() string {
	return "unpartitioned"
}

// WithTaskEffectHandler registers a TaskEffect handler that supports async result retrieval.
func WithTaskEffectHandler[R any](
	ctx context.Context,
	bufferSize int,
) (context.Context, func() context.Context) {
	ctx, endOfTaskHandler := effects.WithResumableEffectHandler(
		ctx,
		bufferSize,
		effectmodel.EffectTask,
		func(ctx context.Context, asyncFn TaskPayload[R]) (R, error) {
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

// TaskEffect performs an asynchronous task and returns a channel with the result.
func TaskEff[R any](ctx context.Context, payload TaskPayload[R]) <-chan handlers.ResumableResult[R] {
	return effects.PerformResumableEffect[TaskPayload[R], R](ctx, effectmodel.EffectTask, payload)
}
