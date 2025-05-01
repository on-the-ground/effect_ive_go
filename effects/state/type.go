package state

import "fmt"

var _ Payload = Source{}

// Source is a special payload type for the state effect handler.
type Source struct{}

func (Source) PartitionKey() string         { return "" }
func (Source) sealedInterfaceStatePayload() {}

var _ Payload = Load{}

// Load is the payload type for retrieving a value from the state.
type Load struct {
	Key any // should be comparable
}

// PartitionKey returns the partition key for routing this payload.
func (p Load) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}

// sealedInterfaceStatePayload prevents external packages from implementing statePayload.
func (p Load) sealedInterfaceStatePayload() {}

var _ Payload = CompareAndDelete{}

// CompareAndDelete is the payload type for deleting a key from the state.
type CompareAndDelete struct {
	Key any // should be comparable
	Old any // should be comparable
}

func (p CompareAndDelete) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p CompareAndDelete) sealedInterfaceStatePayload() {}

var _ Payload = Store{}

// Store is the payload type for deleting a key from the state.
type Store struct {
	Key any // should be comparable
	New any // should be comparable
}

func (p Store) PartitionKey() string {
	return fmt.Sprintf("%v", p.Key)
}
func (p Store) sealedInterfaceStatePayload() {}

// CompareAndSwap is the payload type for inserting or updating a key-value pair.
type CompareAndSwap struct {
	Key any // should be comparable
	New any // should be comparable
	Old any // should be comparable
}

func (p CompareAndSwap) PartitionKey() string         { return fmt.Sprintf("%v", p.Key) }
func (p CompareAndSwap) sealedInterfaceStatePayload() {}

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
