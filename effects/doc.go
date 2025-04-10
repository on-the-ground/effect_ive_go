// Package effects provides a minimal and idiomatic effect system for Go.
//
// Effect-ive Go introduces the Effect Pattern to isolate and delegate side effects
// — such as logging, state, concurrency, and configuration — to dedicated handlers,
// while keeping your business logic pure, testable, and reusable.
//
// # What is an Effect?
//
// An effect is any logic that:
//   - depends on runtime context,
//   - causes external interaction,
//   - or violates pure function guarantees (Who, What, When, Where).
//
// # Why use Effect-ive Go?
//
// Go doesn’t support sum types or native effect systems, but it does offer
// goroutines, channels, and context. Effect-ive Go leverages these idioms
// to implement a scoped, explicit, and panic-safe system for effect delegation.
//
// Benefits include:
//   - Predictable control flow (no hidden side effects)
//   - Testable logic (handlers can be mocked or omitted)
//   - Reusable core logic (no coupling to runtime)
//   - Explicit scoping and handler lifecycle via context
//
// # How does it work?
//
// Handlers are registered via `WithXxxEffectHandler(ctx)` and perform effects
// through `PerformResumableEffect`, `FireAndForgetEffect`, etc.
// Delegation is type-safe, scope-bound, and never implicit.
//
// This package exports:
//   - Built-in handlers (log, state, binding, concurrency)
//   - Reusable helper types (payloads, enums, handler interfaces)
//   - Core effect system to define your own
//
// # Design Philosophy
//
// Effect-ive Go embraces:
//   - Separation of concerns via handler isolation
//   - Simplicity, clarity, and lifecycle safety
//   - The Zen of Go: maintainability over magic
//
// For more: see README.md or visit the Effect-ive Go GitHub page.
//
// Example:
//
//	func handler(ctx context.Context) {
//	    ctx, end := effects.WithStateEffectHandler(ctx, nil)
//	    defer end()
//
//	    effects.StateEffect(ctx, effects.SetStatePayload{Key: "foo", Value: "bar"})
//	}
package effects
