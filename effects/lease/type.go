package lease

type payload interface {
	sealedInterface()
}

// DeregisterOf creates a payload for deregistering a lease resource.
func DeregisterOf(key string) payload {
	return deregister{
		Key: key,
	}
}

// RegisterOf creates a payload for registering a lease resource with a specified number of owners.
func RegisterOf(key string, numOwners int) payload {
	return register{
		Key:       key,
		NumOwners: numOwners,
	}
}

// AcquireOf creates a payload for acquiring a lease resource.
func AcquireOf(key string) payload {
	return acquire{
		Key: key,
	}
}

// ReleaseOf creates a payload for releasing a lease resource.
func ReleaseOf(key string) payload {
	return release{
		Key: key,
	}
}

type register struct {
	Key       string
	NumOwners int
}

func (register) sealedInterface() {}

type deregister struct {
	Key string
}

func (deregister) sealedInterface() {}

type acquire struct {
	Key string
}

func (acquire) sealedInterface() {}

type release struct {
	Key string
}

func (release) sealedInterface() {}

// SourceSinkPair is a structure that holds two channels: source and sink.
type SourceSinkPair[T any] struct {
	source, sink chan T
}

// newFilterablePair creates a new SourceSinkPair with a buffered source channel of size numOwners.
// The sink channel is unbuffered.
// The source channel is used for acquiring leases, while the sink channel is used for releasing them.
// The source channel is filtered to remove expired leases based on the provided expiration function.
// The sink channel is used to release leases.
func newFilterablePair[T any](numOwners int) *SourceSinkPair[T] {
	return &SourceSinkPair[T]{
		source: make(chan T, numOwners),
		sink:   make(chan T),
	}
}

// newBypassPair creates a new SourceSinkPair with a buffered channel of size numOwners for both source and sink.
// This is used when no filtering is needed, and both channels are used for acquiring and releasing leases.
// The source and sink channels are the same, allowing for direct communication between them.
func newBypassPair[T any](numOwners int) *SourceSinkPair[T] {
	ch := make(chan T, numOwners)
	return &SourceSinkPair[T]{
		source: ch,
		sink:   ch,
	}
}
