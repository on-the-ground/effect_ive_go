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

type StateRepo interface {
	// stateRepo is a marker method to prevent accidental implementation of StateRepo directly.
	stateRepo()
}

type CasRepo[K comparable] interface {
	Load(key K) (value any, ok bool)
	InsertIfAbsent(key K, value any) (inserted bool)
	CompareAndSwap(key K, old, new any) (swapped bool)
	CompareAndDelete(key K, old any) (deleted bool)
}

type casRepo[K comparable] interface {
	CasRepo[K]
	stateRepo()
}

type casImpl[K comparable] struct {
	CasRepo[K]
}

func (casImpl[K]) stateRepo() {}

func NewCasRepo[K comparable](repo CasRepo[K]) StateRepo {
	return casImpl[K]{CasRepo: repo}
}

type SetRepo[K comparable] interface {
	Get(key K) (value any, ok bool)
	Set(key K, value any)
	Delete(key K)
}

type setRepo[K comparable] interface {
	SetRepo[K]
	stateRepo()
}

type setImpl[K comparable] struct {
	SetRepo[K]
}

func (setImpl[K]) stateRepo() {}

func NewSetRepo[K comparable](repo SetRepo[K]) StateRepo {
	return setImpl[K]{SetRepo: repo}
}

func matchRepo[K comparable, T any](
	repo StateRepo,
	casCallback func(casRepo[K]) T,
	setCallback func(setRepo[K]) T,
) T {
	switch r := repo.(type) {
	case casRepo[K]:
		return casCallback(r)
	case setRepo[K]:
		return setCallback(r)
	default:
		panic(fmt.Sprintf("exhaustive match fallback, repo type: %T", repo))
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
