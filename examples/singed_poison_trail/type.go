package main

import "sync/atomic"

type Coordinate struct {
	x atomic.Int32
}

func (c *Coordinate) X() int32 {
	return c.x.Load()
}

func (c *Coordinate) Compare(i *Coordinate) bool {
	return c.x.Load() == i.x.Load()
}

func (c *Coordinate) CompareAndSwap(old, new *Coordinate) bool {
	return c.x.CompareAndSwap(old.X(), new.X())
}

func (c *Coordinate) String() string {
	return string(c.x.Load())
}

func newCoordinate(x int32) *Coordinate {
	ret := &Coordinate{
		x: atomic.Int32{},
	}
	ret.x.Store(x)
	return ret
}

type HP struct {
	val int32
}

func newHP(val int32) *HP {
	return &HP{
		val: val,
	}
}
