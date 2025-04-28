package stream

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"

	"github.com/on-the-ground/effect_ive_go/effects/internal/orderedbuffer"
)

type streamEffectPayload interface {
	sealedStreamEffectPayload()
}

type MapStreamPayloadAny interface {
	Run(ctx context.Context)
}

type MapStreamPayload[T any, R any] struct {
	Source <-chan T
	Sink   chan<- R
	MapFn  func(T) R
}

func (m MapStreamPayload[T, R]) Run(ctx context.Context) {
	defer close(m.Sink)
	mapFn(ctx, m.Source, m.Sink, m.MapFn)
}

func (m MapStreamPayload[T, R]) sealedStreamEffectPayload() {}

type FilterStreamPayload[T any] struct {
	Source    <-chan T
	Sink      chan<- T
	Predicate func(T) bool
}

func (f FilterStreamPayload[T]) sealedStreamEffectPayload() {}

type MergeStreamPayload[T any] struct {
	Sources []<-chan T
	Sink    chan<- T
}

func (p MergeStreamPayload[T]) sealedStreamEffectPayload() {}

type SubscribeStreamPayload[T any] struct {
	Source SourceAsKey[T]
	Target *sinkDropPair[T]
}

func (p SubscribeStreamPayload[T]) sealedStreamEffectPayload() {}
func (p SubscribeStreamPayload[T]) PartitionKey() string {
	return p.Source.String()
}

type OrderByStreamPayload[T any] struct {
	WindowSize int
	CmpFn      orderedbuffer.CompareFunc[T]
	Source     SourceAsKey[T]
	Sink       chan<- T
}

func (p OrderByStreamPayload[T]) sealedStreamEffectPayload() {}
func (p OrderByStreamPayload[T]) PartitionKey() string {
	return p.Source.String()
}

type SourceAsKey[T any] <-chan T

func (s SourceAsKey[T]) String() string {
	return fmt.Sprintf("%p", s)
}

type sinkDropPair[T any] struct {
	Sink    chan<- T
	Dropped chan<- T
	closed  atomic.Bool
}

var MinCapacityOfDroppedChannel = 1000

func NewSinkDropPair[T any](sink chan<- T, dropped chan<- T) *sinkDropPair[T] {
	if sink == nil || dropped == nil {
		panic("nil channel not allowed for sink/dropped")
	}
	if channelCapacity(dropped) < MinCapacityOfDroppedChannel {
		panic(fmt.Sprintf("dropped channl should have its capacity larger than %v", MinCapacityOfDroppedChannel))
	}
	return &sinkDropPair[T]{
		Sink:    sink,
		Dropped: dropped,
		closed:  atomic.Bool{},
	}
}

type RegisteredList[T any] struct {
	Registered []*sinkDropPair[T]
}

func channelCapacity(ch interface{}) int {
	val := reflect.ValueOf(ch)
	if val.Kind() != reflect.Chan {
		return -1
	}
	return val.Cap()
}
