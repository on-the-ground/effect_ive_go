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
	coord *Coordinate
	to    *Coordinate
	speed int32
	cb    func(*Coordinate)
}

var nextCoord = purefn.TableizeI3O1(
	func(cur, to *Coordinate, speed int32) (next *Coordinate) {
		remaining := to.X() - cur.X()
		if speed <= remaining {
			return newCoordinate(cur.X() + speed)
		} else if -speed < remaining && remaining < speed {
			return newCoordinate(cur.X() + remaining)
		} else /* remaining <= -m.speed */ {
			return newCoordinate(cur.X() + -1*speed)
		}
	},
	8,
)

func (m *move) TickCallback(ctx context.Context) {
	cur := m.coord
	nxt := nextCoord(cur, m.to, m.speed)
	m.coord.CompareAndSwap(cur, nxt)
	m.cb(m.coord)
}

func (m *move) Done() bool {
	return m.coord.Compare(m.to)
}

func NewMove(coord, to *Coordinate, speed int32, cb func(*Coordinate)) Job {
	return &move{
		coord: coord,
		to:    to,
		speed: speed,
		cb:    cb,
	}
}

type adjustHP struct {
	coord *Coordinate
	hp    *HP
}

func (r *adjustHP) TickCallback(ctx context.Context) {
	prefix := keyOfAdjustHp(r.coord)

	adjustRatePairs, err := state.LoadEffects[float64](ctx, prefix)
	if err != nil {
		log.Effect(ctx, log.LogError, "fail to LoadEffect", map[string]interface{}{
			"key":   prefix,
			"value": true,
			"err":   err,
		})
		return
	}
	adjustRate := 1.0
	for _, rate := range adjustRatePairs {
		adjustRate *= rate
	}
	r.hp.val = int32(float64(r.hp.val) * adjustRate)
}

func (r *adjustHP) Done() bool {
	return false
}

func NewAdjustHP(coord *Coordinate, hp *HP) Job {
	return &adjustHP{
		coord: coord,
		hp:    hp,
	}
}
