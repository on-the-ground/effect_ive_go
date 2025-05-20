package main

import (
	"context"

	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/effects/state"
	"github.com/on-the-ground/effect_ive_go/purefn"
)

type Job interface {
	TickCallback(ctx context.Context)
	Done() bool
}

type move struct {
	*Coordinate
	to, speed int32
	cb        func(*Coordinate)
}

var movement = purefn.TableizeI3O1(
	func(cur, to, speed int32) int32 {
		remaining := to - cur
		if speed <= remaining {
			return speed
		} else if -speed < remaining && remaining < speed {
			return remaining
		} else /* remaining <= -m.speed */ {
			return -1 * speed
		}
	},
	8,
)

func (m *move) TickCallback(ctx context.Context) {
	cur := m.x.Load()
	delta := movement(cur, m.to, m.speed)
	m.x.CompareAndSwap(cur, cur+delta)
	m.cb(m.Coordinate)
}

func (m *move) Done() bool {
	return m.x.Load() == m.to
}
func newMove(position *Coordinate, to, speed int32, callbacks ...func(*Coordinate)) Job {
	if len(callbacks) != 1 {
		panic("only one callback is allowed")
	}
	return &move{
		Coordinate: position,
		to:         to,
		speed:      speed,
		cb:         callbacks[0],
	}
}

type rebalanceHP struct {
	*Coordinate
	hp *HP
}

func (r *rebalanceHP) TickCallback(ctx context.Context) {
	pos := r.x.Load()
	prefix := keyOfRebalanceHp(pos)

	rebalanceRatePairs, err := state.LoadEffects[float64](ctx, prefix)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to LoadEffect", map[string]interface{}{
			"key":   prefix,
			"value": true,
			"err":   err,
		})
		return
	}
	rebalanceRate := 1.0
	for _, rate := range rebalanceRatePairs {
		rebalanceRate *= rate
	}
	r.hp.val = int32(float64(r.hp.val) * rebalanceRate)
}
func (r *rebalanceHP) Done() bool {
	return false
}
