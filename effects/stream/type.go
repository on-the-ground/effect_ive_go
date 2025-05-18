package stream

import (
	"fmt"
)

type subscribePayload[T any] struct {
	Source SourceAsKey[T]
	Target *sinkDropPair[T]
}

func (p subscribePayload[T]) PartitionKey() string {
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
