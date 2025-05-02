package state

import "fmt"

var _ Payload = Source{}

// Source is a special payload type for the state effect handler.
type Source struct{}

func (Source) PartitionKey() string { return "" }
func (Source) payload()             {}

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

// payload prevents external packages from implementing statePayload.
func (p Load[K]) payload() {}

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
func (p CompareAndDelete[K, V]) payload() {}

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
func (p Store[K, V]) payload() {}

// CompareAndSwap is the payload type for inserting or updating a key-value pair.
type CompareAndSwap[K comparable, V comparable] struct {
	Key K // should be comparable
	New V // should be comparable
	Old V // should be comparable
}

func NewCompareAndSwap[K, V comparable](k K, old, new V) Payload {
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

type CasRepo interface {
	Load(key any) (value any, ok bool)
	Store(key, value any)
	CompareAndSwap(key, old, new any) (swapped bool)
	CompareAndDelete(key, old any) (deleted bool)
}

type casRepo interface {
	CasRepo
	stateRepo()
}

type casImpl struct {
	CasRepo
}

func (casImpl) stateRepo() {}

func NewCasRepo(repo CasRepo) StateRepo {
	return casImpl{CasRepo: repo}
}

type SetRepo interface {
	Get(key any) (value any, ok bool)
	Set(key, value any)
	Delete(key any)
}

type setRepo interface {
	SetRepo
	stateRepo()
}

type setImpl struct {
	SetRepo
}

func (setImpl) stateRepo() {}

func NewSetRepo(repo SetRepo) StateRepo {
	return setImpl{SetRepo: repo}
}

func matchRepo[T any](
	repo StateRepo,
	casCallback func(casRepo) T,
	setCallback func(setRepo) T,
) T {
	if casRepo, ok := repo.(casRepo); ok {
		return casCallback(casRepo)
	}
	if setRepo, ok := repo.(setRepo); ok {
		return setCallback(setRepo)
	}
	panic(fmt.Sprintf("exhaustive match fallback, repo type: %T", repo))
}
