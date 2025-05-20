package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/concurrency"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/effects/stream"
)

type HP struct {
	val int32
}

type Singed struct {
	coordinate  *Coordinate
	jobs        []Job
	closed      atomic.Bool
	closeFn     func()
	poisonTrail bool
}

func (s *Singed) TogglePoisonTrail() {
	s.poisonTrail = !s.poisonTrail
}

func trailPoison(ctx context.Context, x int32) {
	key := keyOfRebalanceHp(x) + fmt.Sprintf("/%d", time.Now().UnixNano())
	reductionRate := 0.8

	inserted, err := state.InsertIfAbsentWithTTLEffect(ctx, key, reductionRate, 325*time.Millisecond)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to InsertIfAbsentWithTTLEffect", map[string]interface{}{
			"key":   key,
			"value": true,
			"err":   err,
		})
		return
	}
	if inserted {
		return
	}

}

func (s *Singed) Move(ctx context.Context, to, speed int32) {
	s.jobs = append(s.jobs, newMove(s.coordinate, to, speed, func(p *Coordinate) {
		if s.poisonTrail {
			trailPoison(ctx, p.x.Load())
		}
	}))

}

func (s *Singed) Close() {
	if swapped := s.closed.CompareAndSwap(false, true); !swapped {
		return
	}
	s.closeFn()
}

func NewSinged(
	ctx context.Context,
	tickSource <-chan time.Time,
	droppedTick chan<- time.Time,
) *Singed {
	tickCh := make(chan time.Time, 2)
	stream.SubscribeEffect(ctx, tickSource, tickCh, droppedTick)

	singed := &Singed{
		closeFn: func() {
			stream.UnsubscribeEffect(ctx, tickSource, tickCh)
			close(tickCh)
		},
	}

	concurrency.Effect(ctx, func(ctx context.Context) {
		for {
			select {
			case _, ok := <-tickCh:
				if !ok {
					return
				}

				newJobs := []Job{
					&rebalanceHP{
						Coordinate: singed.coordinate,
						hp:         &HP{val: 100},
					},
				}
				for _, job := range singed.jobs {
					if job.Done() {
						continue
					}
					job.TickCallback(ctx)
					newJobs = append(newJobs, job)
				}
				singed.jobs = newJobs

			case <-ctx.Done():
				return
			}
		}
	})

	return singed
}
