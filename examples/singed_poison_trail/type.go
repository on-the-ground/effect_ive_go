package main

import "sync/atomic"

type Coordinate struct {
	x atomic.Int32
}
