package stream

import (
	"context"
	"fmt"

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
	Sink   chan<- T
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

type SinkList[T any] struct {
	List []chan<- T
}
