// Package dependency provides an effect for interface-based dependency resolution
// using runtime duck typing.
//
// A handler can register a list of objects. When an effect is performed, the
// handler attempts to match the requested interface type against its registered
// dependencies. If a match is found, the handler delegates the method call
// using the provided Quacker implementation. Otherwise, the request is delegated
// upward through the context chain.
//
// This enables dynamic, testable, and scoped dependency injection without using
// global containers or code generation.
package dependency

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/on-the-ground/effect_ive_go/effects"
	"github.com/on-the-ground/effect_ive_go/effects/internal/handlers"
	"github.com/on-the-ground/effect_ive_go/effects/log"
	"github.com/on-the-ground/effect_ive_go/shared/helper"
)

// Exported so consumers can refer to the handler key.
const effectKey effects.EffectKey = "github.com/on-the-ground/effect_ive_go/effects/dependency/effectKey"

// WithEffectHandler registers a dependency effect handler into the context.
// The handler can resolve method calls dynamically by matching interface types
// (duck) to the given list of concrete dependency objects.
//
// Each time a matching dependency is found, the provided Quacker will be invoked
// with that object as the receiver.
//
// This allows dynamic, runtime dependency injection for interface-based dispatch.
//
// Returns the new context and a teardown function to remove the handler.
func WithEffectHandler(
	pctx context.Context,
	bufferSize int,
	dependencies []any,
) (context.Context, func() context.Context) {
	ctx, endOfHandler := effects.WithResumableEffectHandler(
		pctx,
		bufferSize,
		effectKey,
		func(ctx context.Context, pl payload) (any, error) {

			for _, dep := range dependencies {
				if !implements(dep, pl.duck) {
					continue
				}
				log.Effect(ctx, log.LogDebug, "[dependency]", map[string]interface{}{
					"matched":  fmt.Sprintf("%T", dep),
					"expected": pl.duck,
				})
				return pl.quacker.WithReceiver(dep).Quack(ctx)
			}

			res := <-delegateDependencyEffect(ctx, pl)
			return res.Value, res.Err
		},
	)

	return ctx, endOfHandler
}

// Quacker represents an abstraction for deferred method invocation.
// It encapsulates a parsed method signature and arguments, and provides
// a way to dynamically invoke that method on a given receiver.
type Quacker interface {
	WithReceiver(receiver any) Quacker
	Quack(ctx context.Context) (any, error)
}

// Effect performs a resumable dependency resolution effect.
// The generic type parameter D should be the interface type to match.
//
// The given Quacker will be invoked against a matched dependency that implements D.
// If no handler exists in the context, the call panics unless recovered via delegateDependencyEffect.
func Effect[D any](ctx context.Context, quacker Quacker) <-chan handlers.ResumableResult {
	key := reflect.TypeOf((*D)(nil)).Elem()

	pl := payload{
		duck:    key,
		quacker: quacker,
	}

	return effects.PerformResumableEffect(ctx, effectKey, pl)
}

// payload is the internal structure passed as the effect payload.
// It includes the desired interface type (duck) and the Quacker for execution.
type payload struct {
	duck    reflect.Type
	quacker Quacker
}

func (p payload) PartitionKey() string {
	return fmt.Sprintf("%v", p.duck)
}

// implements checks whether the given dependency object satisfies the requested
// interface type (duck), accounting for pointer types and compatibility across
// Go versions.
func implements(dep any, duck reflect.Type) bool {
	depType := reflect.TypeOf(dep)

	if depType.Kind() != reflect.Ptr {
		// resolve PointerTo once (Go 1.21 이하 호환)
		var ptrTo func(reflect.Type) reflect.Type
		if p := reflect.ValueOf(reflect.PointerTo); p.IsValid() {
			ptrTo = reflect.PointerTo // Go 1.22+
		} else {
			ptrTo = reflect.PtrTo // Go <= 1.21
		}
		depType = ptrTo(depType)
	}

	return depType.Implements(duck)
}

// delegateDependencyEffect attempts to forward the dependency resolution
// to parent handlers in the context chain.
//
// If no handler is found, it recovers from the panic and returns a single-element
// channel containing the "no handler" error.
//
// This fallback allows the top-level handler to gracefully handle unresolvable effects.
func delegateDependencyEffect(pctx context.Context, pl payload) (ch <-chan handlers.ResumableResult) {
	defer func() {
		if r := recover(); r != nil {
			if r, ok := r.(error); ok && errors.Is(r, helper.ErrNoEffectHandler) {
				_ch := make(chan handlers.ResumableResult, 1)
				_ch <- handlers.ResumableResultFrom(nil, r)
				ch = _ch
			} else {
				panic(r)
			}
		}
	}()

	return effects.PerformResumableEffect(pctx, effectKey, pl)
}
