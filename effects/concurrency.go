package effects

import (
	"context"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
)

// spawnConcurrentChildren starts each function in its own goroutine with its own context.
// - Each child gets its own cancellable context.
// - Adds the goroutine to the WaitGroup for tracking.
// - Catches and logs panics individually.
func spawnConcurrentChildren(parentContext context.Context, wg *sync.WaitGroup, functions []func(context.Context)) {
	ready := sync.WaitGroup{}
	numRoutines := len(functions)
	childrenCancels := make([]context.CancelFunc, numRoutines)

	for idx, fn := range functions {
		childCtx, cancel := context.WithCancel(context.Background())
		childrenCancels[idx] = cancel
		wg.Add(1)
		ready.Add(1)
		go func(f func(context.Context), ctx context.Context) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					LogEffect(parentContext, LogError, "panic in child routine", map[string]interface{}{
						"routine": f,
						"error":   r,
					})
				}
			}()
			ready.Done()
			f(ctx)
		}(fn, childCtx)
	}

	wg.Add(1)
	ready.Add(1)
	go func() {
		defer wg.Done()
		ready.Done()
		<-parentContext.Done()
		LogEffect(parentContext, LogInfo, "context cancelled, waiting for all routines to finish", nil)
		for _, cancelFn := range childrenCancels {
			cancelFn()
		}
	}()

	ready.Wait()
}

// waitChildren blocks until all child goroutines complete or the context is cancelled.
// - If cancelled, invokes all child cancel functions to propagate cancellation.
func waitChildren(ctx context.Context, wg *sync.WaitGroup) {
	waitCh := make(chan struct{})
	ready := make(chan struct{})
	go func() {
		close(ready)
		LogEffect(ctx, LogInfo, "waiting for all routines to finish", nil)
		wg.Wait()
		close(waitCh)
	}()
	<-ready

	<-waitCh
	LogEffect(ctx, LogInfo, "all routines finished", nil)
}

// WithConcurrencyEffectHandler installs a fire-and-forget concurrency effect handler.
//
// It allows `ConcurrencyEffect(ctx, [...])` to spawn multiple goroutines under managed scope.
//
// - Buffer size is configurable via binding effect.
// - WaitGroup + cancellation tracking ensures children are joined on shutdown.
// - Worker count is fixed to 1 (non-partitioned).
func WithConcurrencyEffectHandler(
	ctx context.Context,
	bufferSize int,
) (context.Context, func() context.Context) {
	const numWorkers = 1 // number of workers is not configurable for concurrency effect

	wg := &sync.WaitGroup{}

	ctx, endOfConcurrency := WithFireAndForgetEffectHandler(
		ctx,
		effectmodel.NewEffectScopeConfig(bufferSize, numWorkers),
		effectmodel.EffectConcurrency,
		func(ctx context.Context, payload []func(context.Context)) {
			spawnConcurrentChildren(ctx, wg, payload)
		},
		func() {
			waitChildren(ctx, wg)
		},
	)

	return ctx, endOfConcurrency
}

// WithConcurrencyEffectHandler installs a fire-and-forget concurrency effect handler.
//
// It allows `ConcurrencyEffect(ctx, [...])` to spawn multiple goroutines under managed scope.
//
// - Buffer size is configurable via binding effect.
// - WaitGroup + cancellation tracking ensures children are joined on shutdown.
// - Worker count is fixed to 1 (non-partitioned).
func ConcurrencyEffect(ctx context.Context, payload []func(context.Context)) {
	FireAndForgetEffect(ctx, effectmodel.EffectConcurrency, payload)
}
