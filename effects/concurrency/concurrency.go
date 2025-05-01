package concurrency

import (
	"context"
	"sync"

	"github.com/on-the-ground/effect_ive_go/effects"
	effectmodel "github.com/on-the-ground/effect_ive_go/effects/internal/model"
	"github.com/on-the-ground/effect_ive_go/effects/log"
)

// WithConcurrencyEffectHandler installs a fire-and-forget concurrency effect handler.
//
// It allows `ConcurrencyEff(ctx, [...])` to spawn multiple goroutines under managed scope.
//
//   - Buffer size is configurable via binding effect.
//   - WaitGroup + cancellation tracking ensures children are joined on shutdown.
//   - Worker count is fixed to 1 (non-partitioned).
//   - Returns a function to end the concurrency effect handler.
//   - The returned function should be called when the effect handler is no longer needed.
//   - If the returned function is called early, the effect handler will be closed,
//     and you should use the context returned by the teardown function.
//   - The returned context is the same as the input context.
func WithConcurrencyEffectHandler(
	ctx context.Context,
	bufferSize int,
) (context.Context, func() context.Context) {
	sv := &supervisor{
		wg:              &sync.WaitGroup{},
		childrenCancels: make([]context.CancelFunc, 0),
		doneCh:          make(chan struct{}),
		readyCh:         make(chan struct{}),
	}
	sv.watchParentCancel(ctx)

	ctx, endOfConcurrencyHandler := effects.WithFireAndForgetEffectHandler(
		ctx,
		bufferSize,
		effectmodel.EffectConcurrency,
		sv.spawnConcurrentChildren,
		func() {
			sv.waitChildren(ctx)
			close(sv.doneCh)
		},
	)

	return ctx, endOfConcurrencyHandler
}

// WithConcurrencyEffectHandler installs a fire-and-forget concurrency effect handler.
//
// It allows `ConcurrencyEff(ctx, [...])` to spawn multiple goroutines under managed scope.
//
// - Buffer size is configurable via binding effect.
// - WaitGroup + cancellation tracking ensures children are joined on shutdown.
// - Worker count is fixed to 1 (non-partitioned).
func ConcurrencyEff(ctx context.Context, fns ...func(context.Context)) {
	effects.FireAndForgetEffect[ConcurrencyPayload](ctx, effectmodel.EffectConcurrency, fns)
}

type ConcurrencyPayload []func(context.Context)

func (cp ConcurrencyPayload) PartitionKey() string {
	return "unpartitioned"
}

// supervisor manages the lifecycle of child goroutines spawned by the concurrency effect handler.
// It tracks the cancellation functions for each child context and ensures
// that all child goroutines are properly cleaned up when the parent context is cancelled.
// It also provides a mechanism to wait for all child goroutines to finish before
// closing the effect handler.
// The supervisor is responsible for:
// - Watching the parent context for cancellation.
// - Spawning child goroutines with their own cancellable contexts.
// - Tracking the cancellation functions for each child context.
// - Waiting for all child goroutines to finish before closing the effect handler.
// - Handling panics in child goroutines and logging them.
// - Ensuring that the effect handler is closed properly when no longer needed.
// - Providing a way to wait for all child goroutines to finish before closing the effect handler.
type supervisor struct {
	wg              *sync.WaitGroup
	childrenCancels []context.CancelFunc
	doneCh          chan struct{}
	readyCh         chan struct{}
}

// watchParentCancel monitors the parent context for cancellation.
// If the parent context is cancelled, it invokes all child cancel functions
// to propagate the cancellation to all child goroutines.
func (s *supervisor) watchParentCancel(parentContext context.Context) {
	select {
	case <-s.readyCh:
	default:
		go func() {
			close(s.readyCh)
			select {
			case <-parentContext.Done():
				log.LogEff(parentContext, log.LogInfo, "context cancelled, waiting for all routines to finish", nil)
				for _, cancelFn := range s.childrenCancels {
					cancelFn()
				}
			case <-s.doneCh:
			}
		}()
		<-s.readyCh
	}
}

// appendCancel adds a child context's cancellation function to the list of tracked cancels.
func (s *supervisor) appendCancel(cancelFn context.CancelFunc) {
	s.childrenCancels = append(s.childrenCancels, cancelFn)
}

// spawnConcurrentChildren starts each function in its own goroutine with its own context.
// - Each child gets its own cancellable context.
// - Adds the goroutine to the WaitGroup for tracking.
// - Catches and logs panics individually.
func (s *supervisor) spawnConcurrentChildren(
	parentContext context.Context,
	functions ConcurrencyPayload,
) {
	ready := sync.WaitGroup{}

	for _, fn := range functions {
		childCtx, cancel := context.WithCancel(context.Background())
		s.appendCancel(cancel)
		s.wg.Add(1)
		ready.Add(1)
		go func(f func(context.Context), ctx context.Context) {
			defer s.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.LogEff(parentContext, log.LogError, "panic in child routine", map[string]interface{}{
						"routine": f,
						"error":   r,
					})
				}
			}()
			ready.Done()
			f(ctx)
		}(fn, childCtx)
	}

	// Wait until all child goroutines have been started before returning
	ready.Wait()
}

// waitChildren blocks until all child goroutines complete or the context is cancelled.
// - If cancelled, invokes all child cancel functions to propagate cancellation.
func (s *supervisor) waitChildren(ctx context.Context) {
	waitCh := make(chan struct{})
	ready := make(chan struct{})
	go func() {
		close(ready)
		log.LogEff(ctx, log.LogInfo, "waiting for all routines to finish", nil)
		s.wg.Wait()
		close(waitCh)
	}()
	<-ready

	<-waitCh
	log.LogEff(ctx, log.LogInfo, "all routines finished", nil)
}
