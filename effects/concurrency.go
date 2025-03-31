package effects

import (
	"context"
	"sync"

	effectmodel "github.com/on-the-ground/effect_ive_go/effects/model"
)

func spawnConcurrentChildren(wg *sync.WaitGroup, childrenCancels *[]context.CancelFunc, functions []func(context.Context)) {
	numRoutines := len(functions)
	*childrenCancels = make([]context.CancelFunc, numRoutines)

	for idx, fn := range functions {
		childCtx, cancel := context.WithCancel(context.Background())
		(*childrenCancels)[idx] = cancel
		wg.Add(1)
		go func(f func(context.Context), ctx context.Context) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					LogEffect(ctx, LogError, "panic in child routine", map[string]interface{}{
						"routine": f,
						"error":   r,
					})
				}
			}()
			f(ctx)
		}(fn, childCtx)
	}
}

func waitChildren(ctx context.Context, wg *sync.WaitGroup, childrenCancels []context.CancelFunc) {
	waitCh := make(chan struct{})
	go func() {
		LogEffect(ctx, LogInfo, "waiting for all routines to finish", nil)
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		LogEffect(ctx, LogInfo, "all routines finished", nil)
	case <-ctx.Done():
		LogEffect(ctx, LogInfo, "context cancelled, waiting for all routines to finish", nil)
		for _, cancelFn := range childrenCancels {
			cancelFn()
		}
	}
}

func WithConcurrencyEffectHandler(
	ctx context.Context,
) (context.Context, func()) {
	wg := &sync.WaitGroup{}
	var childrenCancels []context.CancelFunc

	ctx, endOfConcurrency := WithFireAndForgetEffectHandler(ctx, effectmodel.EffectConcurrency,
		func(ctx context.Context, payload []func(context.Context)) {
			spawnConcurrentChildren(wg, &childrenCancels, payload)
		},
		func() {
			waitChildren(ctx, wg, childrenCancels)
		},
	)

	return ctx, endOfConcurrency
}

func ConcurrencyEffect(ctx context.Context, payload []func(context.Context)) {
	FireAndForgetEffect(ctx, effectmodel.EffectConcurrency, payload)
}
