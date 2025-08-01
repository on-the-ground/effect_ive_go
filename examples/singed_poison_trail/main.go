package main

import (
	"context"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
)

func main() {

	ctx := context.Background()

	tickSrc := time.NewTicker(250 * time.Millisecond)
	defer tickSrc.Stop()

	ctx, endOfConcurrencyHandler := concurrency.WithEffectHandler(ctx, 2)
	defer endOfConcurrencyHandler()

	droppedTick := make(chan time.Time, 1000)
	concurrency.Effect(ctx, func(ctx context.Context) {
		for {
			select {
			case <-droppedTick:
			case <-ctx.Done():
				return
			}
		}
	})

	ctx, endOfStateHandler := state.WithEffectHandler[string, state.ComparableEquatable](
		ctx, 2, 1, false,
		state.NewInMemoryStore[string](),
		nil,
	)
	defer endOfStateHandler()

	ctx, endOfStreamHandler := stream.WithEffectHandler[time.Time](ctx, 2)
	defer endOfStreamHandler()

	singed := NewSinged(ctx, tickSrc.C, droppedTick)
	defer singed.Close()

	singed.Move(ctx, 10, 1)

}
