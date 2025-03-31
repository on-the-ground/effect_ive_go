package effects_test

import (
	"testing"
)

// Each goroutine must maintain its own independent effect scope chain, and must not be able to access effect handlers of other goroutines.
func TestConcurrencyIfErrOnly(t *testing.T) {

}

// Effect propagation should operate in a goroutine-local manner.

// When cancel() is called on a parent scope, all child goroutines must be terminated without exception.

// The lifecycle must be completed safely without any goroutine leaks.

// The supervisor should be able to detect panics and terminate all child goroutines; this behavior should be configurable.
