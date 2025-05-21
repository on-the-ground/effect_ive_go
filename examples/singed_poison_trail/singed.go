package main

import (
	"context"
	"fmt"
	"time"

	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"
)

type Singed struct {
	*Actor
	poisonTrail bool
}

func (s *Singed) TogglePoisonTrail() {
	s.poisonTrail = !s.poisonTrail
}

func trailPoison(ctx context.Context, coord *Coordinate) {
	key := keyOfAdjustHp(coord) + fmt.Sprintf("/%d", time.Now().UnixNano())
	reductionRate := 0.8

	inserted, err := state.InsertIfAbsentWithTTLEffect(ctx, key, reductionRate, 325*time.Millisecond)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to InsertIfAbsentWithTTLEffect", map[string]interface{}{
			"key":   key,
			"value": true,
			"err":   err,
		})
	}
	if !inserted {
		log.Effect(ctx, log.LogInfo, "already exist", map[string]interface{}{
			"key":   key,
			"value": true,
		})
	}
}

func (s *Singed) Move(ctx context.Context, toX int32, speed int32) {
	s.Actor.Move(ctx, newCoordinate(toX), speed, func(coordinate *Coordinate) {
		if s.poisonTrail {
			trailPoison(ctx, coordinate)
		}
	})
}

func NewSinged(
	ctx context.Context,
	tickSource <-chan time.Time,
	droppedTick chan<- time.Time,
) *Singed {
	singedMaxHP := int32(100)
	actor := NewActor(ctx, tickSource, droppedTick, singedMaxHP)
	return &Singed{
		Actor:       actor,
		poisonTrail: false,
	}
}
