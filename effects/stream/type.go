package stream

import (
	"fmt"
)

type payload[T any] interface {
	sealedStreamPayload()
}

type unsubscribePayload[T any] struct {
	Source  SourceAsKey[T]
	Sink    chan<- T
	Dropped chan<- T
}

func (p unsubscribePayload[T]) PartitionKey() string {
	return p.Source.String()
}

func (p unsubscribePayload[T]) sealedStreamPayload() {}

func newUnsubscribePayload[T any](source SourceAsKey[T], sink chan<- T) payload[T] {
	return unsubscribePayload[T]{
		Source: source,
		Sink:   sink,
	}
}

func newSubscribePayload[T any](source SourceAsKey[T], sink, dropped chan<- T) payload[T] {
	return subscribePayload[T]{
		Source:  source,
		Sink:    sink,
		Dropped: dropped,
	}
}

type subscribePayload[T any] struct {
	Source  SourceAsKey[T]
	Sink    chan<- T
	Dropped chan<- T
}

func (p subscribePayload[T]) PartitionKey() string {
	return p.Source.String()
}

func (p subscribePayload[T]) sealedStreamPayload() {}

type SourceAsKey[T any] <-chan T

func (s SourceAsKey[T]) String() string {
	return fmt.Sprintf("%p", s)
}

type sinkDropPair[T any] struct {
	Sink    chan<- T
	Dropped chan<- T
}

func newSinkDropPair[T any](sink chan<- T, dropped chan<- T) *sinkDropPair[T] {
	if sink == nil || dropped == nil {
		panic("nil channel not allowed for sink/dropped")
	}
	return &sinkDropPair[T]{
		Sink:    sink,
		Dropped: dropped,
	}
}

type RegisteredList[T any] struct {
	Registered []*sinkDropPair[T]
}

// func channelCapacity(ch interface{}) int {
// 	val := reflect.ValueOf(ch)
// 	if val.Kind() != reflect.Chan {
// 		return -1
// 	}
// 	return val.Cap()
// }
