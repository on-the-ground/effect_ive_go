package state

import (
	"fmt"
	"reflect"
)

var _ Payload = Source{}

// Source is a special payload type for the state effect handler.
type Source struct{}

func (Source) PartitionKey() string { return "" }
func (Source) payload()             {}

// load is the payload type for retrieving a value from the state.
type load[K comparable] struct {
	Key K // should be comparable
}

func LoadPayloadOf[K comparable](k K) Payload {
	return load[K]{Key: k}
}

// PartitionKey returns the partition key for routing this payload.
func (p load[K]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}

// payload prevents external packages from implementing statePayload.
func (p load[K]) payload() {}

// CompareAndDelete is the payload type for deleting a key from the state.
type CompareAndDelete[K comparable, V ComparableEquatable] struct {
	Key K // should be comparable
	Old V // should be comparable
}

func CADPayloadOf[K comparable, V ComparableEquatable](k K, v V) Payload {
	return CompareAndDelete[K, V]{Key: k, Old: v}
}

func (p CompareAndDelete[K, V]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p CompareAndDelete[K, V]) payload() {}

// InsertIfAbsent is the payload type for deleting a key from the state.
type InsertIfAbsent[K comparable, V ComparableEquatable] struct {
	Key K // should be comparable
	New V // should be comparable
}

func InsertPayloadOf[K comparable, V ComparableEquatable](k K, v V) Payload {
	return InsertIfAbsent[K, V]{Key: k, New: v}
}

func (p InsertIfAbsent[K, V]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p InsertIfAbsent[K, V]) payload() {}

// CompareAndSwap is the payload type for inserting or updating a key-value pair.
type CompareAndSwap[K comparable, V ComparableEquatable] struct {
	Key K // should be comparable
	New V // should be comparable
	Old V // should be comparable
}

func CASPayloadOf[K comparable, V ComparableEquatable](k K, old, new V) Payload {
	return CompareAndSwap[K, V]{Key: k, Old: old, New: new}
}

func (p CompareAndSwap[K, V]) PartitionKey() string { return fmt.Sprintf("%v", p.Key) }
func (p CompareAndSwap[K, V]) payload()             {}

// Payload is a sealed interface for state operations.
// Only predefined payload types (Set, Get, Delete) can implement this interface.
type Payload interface {
	PartitionKey() string
	payload()
}

type StateStore interface {
	// stateStore is a marker method to prevent accidental implementation of StateStore directly.
	stateStore()
}

type CasStore[K comparable] interface {
	Load(key K) (value any, ok bool)
	InsertIfAbsent(key K, value any) (inserted bool)
	CompareAndSwap(key K, old, new any) (swapped bool)
	CompareAndDelete(key K, old any) (deleted bool)
}

type casStore[K comparable] interface {
	CasStore[K]
	stateStore()
}

type casImpl[K comparable] struct {
	CasStore[K]
}

func (casImpl[K]) stateStore() {}

func NewCasStore[K comparable](store CasStore[K]) StateStore {
	return casImpl[K]{CasStore: store}
}

type SetStore[K comparable] interface {
	Get(key K) (value any, ok bool)
	Set(key K, value any)
	Delete(key K)
}

type setStore[K comparable] interface {
	SetStore[K]
	stateStore()
}

type setImpl[K comparable] struct {
	SetStore[K]
}

func (setImpl[K]) stateStore() {}

func NewSetStore[K comparable](store SetStore[K]) StateStore {
	return setImpl[K]{SetStore: store}
}

func matchStore[K comparable, T any](
	store StateStore,
	casCallback func(casStore[K]) T,
	setCallback func(setStore[K]) T,
) T {
	switch r := store.(type) {
	case casStore[K]:
		return casCallback(r)
	case setStore[K]:
		return setCallback(r)
	default:
		panic(fmt.Sprintf("exhaustive match fallback, store type: %T", store))
	}
}

type Equatable interface {
	Equals(j any) bool
}

type ComparableEquatable interface{}

func equalsComparable[T comparable](i, j T) bool {
	return i == j
}

func Equals(i, j ComparableEquatable) bool {
	if reflect.TypeOf(i) != reflect.TypeOf(j) {
		return false
	}
	if i, ok := i.(Equatable); ok {
		return i.Equals(j)
	}
	return equalsComparable(i, j)
}
