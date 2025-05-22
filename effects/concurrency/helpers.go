package concurrency

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
)

// AwaitAll waits for all tasks to complete and returns their results.
func AwaitAll[T any](ctx context.Context, taskChs ...<-chan handlers.ResumableResult[T]) ([]T, []error) {
	numTasks := len(taskChs)
	if numTasks == 0 {
		return nil, nil
	}

	ctx, endOfConcurrencyHandler := WithEffectHandler(ctx, 1)
	defer endOfConcurrencyHandler()

	results := make([]T, numTasks)
	errors := make([]error, numTasks)

	awaitThunkOf := func(i int, taskCh <-chan handlers.ResumableResult[T]) func(ctx context.Context) {
		return func(ctx context.Context) {
			select {
			case result, ok := <-taskCh:
				if !ok {
					return
				}
				results[i] = result.Value
				errors[i] = result.Err
			case <-ctx.Done():
				errors[i] = ctx.Err()
				return
			}
		}
	}

	for i, taskCh := range taskChs {
		Effect(ctx, awaitThunkOf(i, taskCh))
	}

	return results, errors
}
