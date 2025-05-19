package concurrency

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
)

// AwaitAll waits for all tasks to complete and returns their results.
func AwaitAll(ctx context.Context, taskChs ...<-chan handlers.ResumableResult[Payload]) ([]Payload, []error) {
	numTasks := len(taskChs)
	if numTasks == 0 {
		return nil, nil
	}

	ctx, endOfConcurrencyHandler := WithEffectHandler(ctx, 1)
	defer endOfConcurrencyHandler()

	payloads := make([]Payload, numTasks)
	errors := make([]error, numTasks)

	awaitThunkOf := func(i int, taskCh <-chan handlers.ResumableResult[Payload]) func(ctx context.Context) {
		return func(ctx context.Context) {
			select {
			case result, ok := <-taskCh:
				if !ok {
					return
				}
				payloads[i] = result.Value
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

	return payloads, errors
}
