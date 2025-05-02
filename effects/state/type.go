package state

import "fmt"

var _ Payload = Source{}

// Source is a special payload type for the state effect handler.
type Source struct{}

func (Source) PartitionKey() string         { return "" }
func (Source) sealedInterfaceStatePayload() {}

// Load is the payload type for retrieving a value from the state.
type Load[K comparable] struct {
	Key K // should be comparable
}

func NewLoad[K comparable](k K) Payload {
	return Load[K]{Key: k}
}

// PartitionKey returns the partition key for routing this payload.
func (p Load[K]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}

// sealedInterfaceStatePayload prevents external packages from implementing statePayload.
func (p Load[K]) sealedInterfaceStatePayload() {}

// CompareAndDelete is the payload type for deleting a key from the state.
type CompareAndDelete[K, V comparable] struct {
	Key K // should be comparable
	Old V // should be comparable
}

func NewCompareAndDelete[K, V comparable](k K, v V) Payload {
	return CompareAndDelete[K, V]{Key: k, Old: v}
}

func (p CompareAndDelete[K, V]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p CompareAndDelete[K, V]) sealedInterfaceStatePayload() {}

// Store is the payload type for deleting a key from the state.
type Store[K, V comparable] struct {
	Key K // should be comparable
	New V // should be comparable
}

func NewStore[K, V comparable](k K, v V) Payload {
	return Store[K, V]{Key: k, New: v}
}

func (p Store[K, V]) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p Store[K, V]) sealedInterfaceStatePayload() {}

// CompareAndSwap is the payload type for inserting or updating a key-value pair.
type CompareAndSwap[K comparable, V comparable] struct {
	Key K // should be comparable
	New V // should be comparable
	Old V // should be comparable
}

func NewCompareAndSwap[K, V comparable](k K, old, new V) Payload {
	return CompareAndSwap[K, V]{Key: k, Old: old, New: new}
}

func (p CompareAndSwap[K, V]) PartitionKey() string         { return fmt.Sprintf("%v", p.Key) }
func (p CompareAndSwap[K, V]) sealedInterfaceStatePayload() {}

// Payload is a sealed interface for state operations.
// Only predefined payload types (Set, Get, Delete) can implement this interface.
type Payload interface {
	PartitionKey() string
	sealedInterfaceStatePayload()
}

type StateRepo interface {
	Load(key any) (value any, ok bool)
	Store(key, value any)
	CompareAndSwap(key, old, new any) (swapped bool)
	CompareAndDelete(key, old any) (deleted bool)
}

type StateRepo2 interface {
	Get(key any) (value any, ok bool)
	Set(key, value any)
	Delete(key, old any)
}
