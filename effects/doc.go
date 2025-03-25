// Package effects implements a minimal effect handler system for Go.
//
// Inspired by algebraic effects in languages like OCaml, Koka, and Arrow in Kotlin,
// this package explores how effect isolation can be idiomatically expressed in Go,
// using goroutines and channels instead of continuations.
//
// This package is intended as a philosophical and practical supplement to
// https://go.dev/doc/effective_go – where effects are not covered explicitly.
//
// # Overview
//
// The core concepts include:
//
//   - TBD
//
// # Why This Matters
//
// Error handling, logging, dependency injection, and concurrency boundaries are
// often handled inconsistently. By isolating effects, we can test, reason about,
// and reuse logic more safely and expressively.
//
// See examples for usage.
package effects
