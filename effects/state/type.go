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

// Load is the payload type for retrieving a value from the state.
type Load[K comparable] struct {
	Key K // should be comparable
}

// PartitionKey returns the partition key for routing this payload.
func (p Load[K]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}

// payload prevents external packages from implementing statePayload.
func (p Load[K]) payload() {}

// loadWoDelegation is the payload type for retrieving a value from the state without delegation.
type loadWoDelegation[K comparable] struct {
	Key K // should be comparable
}

func (p loadWoDelegation[K]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p loadWoDelegation[K]) payload() {}

// CompareAndDelete is the payload type for deleting a key from the state.
type CompareAndDelete[K comparable, V ComparableEquatable] struct {
	Key K // should be comparable
	Old V // should be comparable
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
	Load(key K) (value any, ok bool, err error)
	InsertIfAbsent(key K, value any) (inserted bool, err error)
	CompareAndSwap(key K, old, new any) (swapped bool, err error)
	CompareAndDelete(key K, old any) (deleted bool, err error)
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
	Get(key K) (value any, ok bool, err error)
	Set(key K, value any) error
	Delete(key K) error
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
	casCallback func(casStore[K]) (T, error),
	setCallback func(setStore[K]) (T, error),
) (T, error) {
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
