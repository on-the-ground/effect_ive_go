package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
)

type Actor struct {
	coordinate *Coordinate
	jobs       []Job
	closed     atomic.Bool
	closeFn    func()
	hp         *HP
}

func normalizeCallback(callbacks []func(*Coordinate)) func(*Coordinate) {
	if len(callbacks) > 1 {
		panic("only one callback is allowed")
	}
	if len(callbacks) == 1 {
		return callbacks[0]
	}
	return nil
}

func (s *Actor) Move(ctx context.Context, to *Coordinate, speed int32, callback ...func(*Coordinate)) {
	s.jobs = append(s.jobs,
		NewMove(s.coordinate, to, speed, normalizeCallback(callback)),
	)
}

func (s *Actor) Close() {
	if swapped := s.closed.CompareAndSwap(false, true); !swapped {
		return
	}
	s.closeFn()
}

func NewActor(
	ctx context.Context,
	tickSource <-chan time.Time,
	droppedTick chan<- time.Time,
	hp int32,
) *Actor {
	tickCh := make(chan time.Time, 2)
	stream.SubscribeEffect(ctx, tickSource, tickCh, droppedTick)

	actor := &Actor{
		coordinate: newCoordinate(0),
		jobs:       []Job{},
		closed:     atomic.Bool{},
		closeFn: func() {
			stream.UnsubscribeEffect(ctx, tickSource, tickCh)
			close(tickCh)
		},
		hp: newHP(hp),
	}

	concurrency.Effect(ctx, func(ctx context.Context) {
		for {
			select {
			case _, ok := <-tickCh:
				if !ok {
					return
				}

				newJobs := []Job{
					NewAdjustHP(actor.coordinate, actor.hp),
				}
				for _, job := range actor.jobs {
					if job.Done() {
						continue
					}
					job.TickCallback(ctx)
					newJobs = append(newJobs, job)
				}
				actor.jobs = newJobs

			case <-ctx.Done():
				return
			}
		}
	})

	return actor
}
