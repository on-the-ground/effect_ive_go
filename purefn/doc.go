// Package purefn provides high-level memoization utilities for pure functions.
//
// Tableize is not just a utility to add memoization.
// Tableize is a tool that *forces the developer to ask*:
//
//	→ "Is this function really pure?"
//	→ "Can this computation be treated as a lazy table?"
//
// That question is not about performance—it's about trust and meaning.
// In this sense, Tableize encourages a more disciplined style of programming.
//
// The centerpiece is the Tableize family of functions, which memoize pure function
// calls by their input values. These functions assume purity—not just determinism,
// but referential transparency—and are designed to be embedded safely within effectful code.
//
// Features:
//   - TableizeI1O1 to TableizeI4O2: Typed, generic memoizers for common arities.
//   - Trie-based bounded cache with dual-map rotation.
//   - Type-safe, zero-reflection, high-performance design.
//   - Compatible with effect_ive_go's effect separation philosophy.
//
// This package embodies the idea that:
//
//	> If a function is pure, it should be cacheable like a mathematical function.
//
// See tableize_test.go and tableize_bench_test.go for usage and benchmarks.
//
// WARNING: Do not use Tableize on impure functions (e.g., those depending on time, I/O, etc).
package purefn
