package stream

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/on-the-ground/effect_ive_go/shared/orderedbuffer"
)

type payload interface {
	sealedStreamEffectPayload()
}

type MapAny interface {
	Run(ctx context.Context)
}

type Map[T any, R any] struct {
	Source <-chan T
	Sink   chan<- R
	MapFn  func(T) R
}

func (m Map[T, R]) Run(ctx context.Context) {
	defer close(m.Sink)
	mapFn(ctx, m.Source, m.Sink, m.MapFn)
}

func (m Map[T, R]) sealedStreamEffectPayload() {}

type EagerFilter[T any] struct {
	Source    <-chan T
	Sink      chan<- T
	Predicate func(T) bool
}

func (f EagerFilter[T]) sealedStreamEffectPayload() {}

type LazyPredicate[T any] struct {
	Predicate    func(T) bool
	PollInterval time.Duration
}

type LazyFilter[T any] struct {
	Source   <-chan T
	Sink     chan<- T
	LazyInfo LazyPredicate[T]
}

func (f LazyFilter[T]) sealedStreamEffectPayload() {}

type Merge[T any] struct {
	Sources []<-chan T
	Sink    chan<- T
}

func (p Merge[T]) sealedStreamEffectPayload() {}

type Subscribe[T any] struct {
	Source SourceAsKey[T]
	Target *sinkDropPair[T]
}

func (p Subscribe[T]) sealedStreamEffectPayload() {}
func (p Subscribe[T]) PartitionKey() string {
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
