package lease

type payload interface {
	sealedInterface()
}

func DeregisterOf(key string) payload {
	return deregister{
		Key: key,
	}
}

func RegisterOf(key string, numOwners int) payload {
	return register{
		Key:       key,
		NumOwners: numOwners,
	}
}

func AcquireOf(key string) payload {
	return acquire{
		Key: key,
	}
}

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
