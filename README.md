<p align="center">
  <img src="https://github.com/user-attachments/assets/bad6f14e-0444-4fbb-ad9d-74131c970568" />
  <br />
  <em>“The Zen of the Effect-ive Gopher” – calm, centered, and side-effect free.</em>
</p>


## Worst Kind of Tests?

- Tests tightly coupled with implementation    
	- Reproduce corner cases and subtle timing issues tied to the current implementation<!-- .element: style="font-size: 80%;" -->
	- Refactoring invalidates tests → Regression<!-- .element: style="font-size: 80%;" -->
	- Code becomes rigid, resistant to change<!-- .element: style="font-size: 80%;" -->
	- Maintaining tests becomes harder than maintaining code<!-- .element: style="font-size: 80%;  margin-bottom: 1em;" -->
- How did we end up testing the implementation instead of the interface?    

---
## Goroutines + Channels / Shared Memory

- Complex entanglement of concurrency, synchronization, and ownership within the implementation      
	- Multiple goroutines with race conditions or timing issues around mutual exclusion and channels<!-- .element: style="font-size: 80%;" -->
	- Using mocks, spies, or exposing internal state -> debugging?<!-- .element: style="font-size: 80%; margin-bottom: 1em;" -->
- CSP is hard to test   
	- Preemptive goroutines<!-- .element: style="font-size: 80%;" -->
	- Implicit synchronization through shared channels<!-- .element: style="font-size: 80%;" -->
	- Non-deterministic select behavior<!-- .element: style="font-size: 80%; margin-bottom: 1em;" -->
- Is there a better alternative?  
---
## Separation of Concerns
- Divide & Conquer: M * N * L -> M + N + L <!-- .element:  style="margin-bottom: 1em;" -->
- Pure functions: Tableizable     
	- Core data transformation logic<!-- .element: style="font-size: 80%;" -->
	- Easy to test, can be skipped if obvious<!-- .element: style="font-size: 80%; margin-bottom: 1em;" -->
- Side effects: Non-tableizable 
	- The devil lies in the effects: concurrency, synchronization, ownership, stream, state<!-- .element: style="font-size: 80%;" -->
	- Mixing pure logic with various side effects leads to the testing hell mentioned earlier <!-- .element: style="font-size: 80%; margin-bottom: 1em;" -->
- Is there a way to keep unavoidable side effects separated from pure logic?    
	- → **Effect Pattern**

---
## Cached Database


``` go
// http://github.com/on-the-ground/effect_ive_go/blob/main/examples/cached_database/main.go
...
	ctx, endOfDBHandler := state.WithEffectHandler[string, Person](
		false, // delegation == false
		state.NewCasStore(memDB),
		...
	)
	defer endOfDBHandler()
...
	ctx, endOfCacheHandler := state.WithEffectHandler[string, Person](
		true, // delegation == true
		state.NewSetStore(rist),
		...
	)
	defer endOfCacheHandler()
...
	ok, err := state.EffectInsertIfAbsent(ctx, key, person)
	log.Effect(ctx, log.LogInfo, "insert attempt", map[string]interface{}{
		"key":     key,
		"value":   person,
		"success": ok,
		"error":   err,
	})
```
---
## [Singed's Poison Trail](https://github.com/on-the-ground/effect_ive_go/tree/main/examples/singed_poison_trail)

---
## Effect-ive Go != MagicBox

<img src="https://github.com/on-the-ground/effect_ive_go/blob/gh-pages/docs/assets/Overview.png" width="440" />

  - Effect-ive Go proposes the minimal idiomatic interface for delegating effects, staying true to Go
	  - Uses `context` and `teardown` for idiomatic effect handler binding/unbinding<!-- .element: style="font-size: 80%;" -->
	  - Effects declared with type and payload<!-- .element: style="font-size: 80%;" -->
	  - Effect payloads are sent over channels to matching handlers found in context<!-- .element: style="font-size: 80%; margin-bottom: 1em;" -->

 
---
## Why Use Context to Find Handlers?
- Explicit ≠ Clear: Does the function signature reveal the core logic?


```go
// Typical function
func ValidateUser(ctx context.Context, db *sql.DB, logger *Logger, metrics *Metrics, config *AppConfig, tracer *Tracer, requestID string, featureFlags map[string]bool, ...) (bool, error)
```

```go
// With injected handlers
func ValidateUser(ctx context.Context, stateHdl StateHandler, logHdl LogHandler, statHdl StatisticsHandler, configHdl ConfigHandler, obsrcHdl ObservationHandler,  ...) (bool, error)
```

```go
// With scoped handlers
// Only core logic dependencies are exposed; auxiliary effects are delegated via context
func ValidateUser(ctx context.Context, whiteList, blackList []User, user User) bool
```

```go
// Context abusing
// All core logic dependencies must be explicitly shown in the function signature
func ValidateUser(ctx context.Context, user User) bool
```
---

# Effects on Effect-ive Go

---

## [Basis](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects)


- Effect categories : Resumable / FireAndForget / ~Abortive(not suitable for Go)~

- Declaring an Effect:
	- Lookup handler by effectKey via context<!-- .element: style="font-size: 80%;" -->
		- Context is only for handler discovery
	- Pass payload via handler channel<!-- .element: style="font-size: 80%;" -->
	- Wait for result (Resumable), or send without waiting (FireAndForget)<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- Handler behavior:
	- Resumable: returns result via resume channel<!-- .element: style="font-size: 80%;" -->
	- Partitionable handlers process in parallel based on partitions<!-- .element: style="font-size: 80%;" -->


## [Dependency](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/dependency)

- The `Dependency` effect implements runtime dependency resolution based on Go interfaces.
	- Handlers act as dynamic dispatchers that **delegate method calls** to objects implementing the requested interface.
	- Objects are matched using **duck typing**: signature-based validation at runtime.

- This enables interface-based inversion of control without relying on DI containers or global registries.

```go
type Dep1 struct{}

func (Dep1) Id() string { return "dep1" }
func (Dep1) Fn1(ctx context.Context) (string, error) {
	return "result from dep1", nil
}

type Dep2 struct{}

func (*Dep2) Id() string { return "dep2" }

type DepINeed interface {
	Id() string
	Fn1(ctx context.Context) (string, error)
}

type idGetter struct {
	DepINeed
	prefix string
}

func (i idGetter) WithReceiver(dependency any) dependency.Quacker {
	i.DepINeed = dependency.(DepINeed)
	return i
}

func (i idGetter) Quack(ctx context.Context) (any, error) {
	if i.DepINeed == nil {
		return nil, errors.New("receiver not set in Quacker")
	}
	return i.prefix + i.DepINeed.Id(), nil
}

func newIdGetter(prefix string) idGetter {
	return idGetter{prefix: prefix}
}

func TestDependencyEffect_Success(t *testing.T) {
	ctx := context.Background()

	ctx, endDep := dependency.WithEffectHandler(ctx, 1, []any{&Dep2{}, Dep1{}})
	defer endDep()

	ch := dependency.Effect[DepINeed](ctx, newIdGetter("test"))
	res := <-ch

	require.NoError(t, res.Err)
	require.Equal(t, "testdep1", res.Value)
}
```
- Delegation is fully supported: if the current handler does not resolve the interface, the effect bubbles up to parent handlers:

- This makes it easy to:
	- Inject test doubles or mocks
	- Override behavior by layer
	- Support context-local DI for concurrent goroutines



## [State](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/state)
- Lock free effects
	- Insert, Load, CompareAndSwap, CompareAndDelete effects<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->

- EventSourcingEffect
	- Subscribe to state command operations via prefix<!-- .element: style="font-size: 80%;" -->

- TTL support
- Multi-tier state using delegation

---

## [Stream](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/stream)

- Stream operators stay alive until source is closed or context is canceled
	- EagerFilter, LazyFilter, Map, Merge, OrderBy, Pipe(bypass)<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->

- Arbiter provided to safely consume from source
	- Subscribe, Unsubscribe<!-- .element: style="font-size: 80%;" -->


---
## [Lease](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/lease)

- Combines Stream and State handlers
- External Semaphore
	- ResourceRegistration, Deregistration, Acquire, Release effects<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->

- TTL  support

```go
	// With numOwners == 1, this acts as a lock and expires after TTL
	ok, err := lease.ResourceRegistrationEffect(ctx, key, 1, ttl, pollInterval)

	ok, err = lease.AcquisitionEffect(ctx, key)

	/* Mutex zone */

	ok, err = lease.ReleaseEffect(ctx, key)

	ok, err = lease.ResourceDeregistrationEffect(ctx, key)
```

---

## [Concurrency](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/concurrency)

- Provides a supervisor for managing goroutines within a scope
	- Parent and children **never** share context<!-- .element: style="font-size: 80%;" -->

	- Cancellation from parent is propagated by supervisor<!-- .element: style="font-size: 80%;" -->

	- All child goroutines are ensured to terminate when scope ends<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->

- `AwaitAll` allows waiting for multiple tasks simultaneously
---

## [Task](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/task)

- Go does not distinguish between sync and async functions
	- CSP enables seamless concurrency<!-- .element: style="font-size: 80%;" -->

	- But without distinction, hangs may happen unpredictably<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->

- Task effect separates async invocation from result retrieval

---

## [Binding](https://pkg.go.dev/github.com/on-the-ground/effect_ive_go/effects/binding)
- State: for managing dynamic key-value data
- Binding: for static, read-only key-value data
- Used for config, environment lookup


---

## Time?

- Time requires precision → not delegable<!-- .element: style="margin-bottom: 1em;" -->
- Is nanosecond-level precision from the runtime meaningful?<!-- .element: style="margin-bottom: 1em;" -->
- Timespan:
	- Treat time not as a point but a range <!-- .element: style="font-size: 80%;" -->
	- Let the system ensure operations occur within the range<!-- .element: style="font-size: 80%;" -->
	- Allows for trust in the span(SoC)<!-- .element: style="font-size: 80%;" -->

---

## Error?

- A brief history of error handling:
	- Output as error: `return -1`<!-- .element: style="font-size: 80%;" -->
	- Clean output with exception: `throw exp`<!-- .element: style="font-size: 80%;" -->
	- Error as output: `return output, error`<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- In Go, errors are outputs
	- - - Attaching contextual messages to errors is part of domain logic<!-- .element: style="font-size: 80%;" -->

---

## Once you've extracted all effects, is what remains truly pure?

---

## Function as Table
<!-- .slide: class="function-as-a-table" -->

- Functions in practical languages can be impure at any time  
	- It's essential to separately identify truly pure parts <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- A pure function is a lazy table  
	- Same input → always the same output <!-- .element: style="font-size: 80%;" -->
	- ~Local reasoning, referential transparency, substitution?~ <!-- .element: style="font-size: 80%;" -->
	- => Tableizable, convertible to a table<!-- .element: style="font-size: 80%;" -->

```go
fib = purefn.TableizeI1O1(func(n int) int {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}, 32)
```

---
## Tableize Implementation
<!-- .slide: class="tableize-impl" -->

```go
func tableize[O any](
	pureFn func(...ComparableOrStringer) O,
	maxTableSize uint32,
) func(...ComparableOrStringer) O {
	memo := NewTrie[O](maxTableSize)
	return func(args ...ComparableOrStringer) O {
		keys := make([]ComparableOrString, len(args))
		for i, arg := range args {
			keys[i] = tableKey(arg)
		}
		v, ok := memo.Load(keys)
		if !ok {
			v = pureFn(args...)
			memo.Store(keys, v)
		}
		return v
	}
}
```
---
## Benchmark

```
cpu: Intel(R) Core(TM) i7-14700

BenchmarkNaiveFib20-28                           55534       19869 ns/op       0 B/op      0 allocs/op

BenchmarkTableizedFib20-28                    24441051       71.02 ns/op      32 B/op      2 allocs/op

BenchmarkNaiveLevenshtein-28                    315432        3809 ns/op       0 B/op      0 allocs/op

BenchmarkTableizedLevenshtein/TrieSize_2-28    7247058       201.3 ns/op      96 B/op      4 allocs/op

BenchmarkTableizedLevenshtein/TrieSize_8-28    5805490       204.2 ns/op      96 B/op      4 allocs/op

BenchmarkTableizedLevenshtein/TrieSize_32-28   7311618       196.4 ns/op      96 B/op      4 allocs/op

BenchmarkNaiveDist-28                       1000000000      0.1012 ns/op       0 B/op      0 allocs/op

BenchmarkTableizedDist-28                      6820208       203.3 ns/op      96 B/op      4 allocs/op

PASS

coverage: 57.3% of statements
```

---
<!-- .slide: class="tableize-debug" -->
## TableizeDebug
- What if you tableized something assuming it was pure, but the output turns out to be unstable? <!-- .element: style="margin-bottom: 1em;" -->
- Use this for validation in CI, testbeds, or canary environments before production deployment


```go
func tableizeDebug[O ComparableEquatable](
	pureFn func(...ComparableOrStringer) O,
	maxTableSize uint32,
) func(...ComparableOrStringer) O {
	memo := NewTrie[O](maxTableSize)
	return func(args ...ComparableOrStringer) O {
		...
		actual = pureFn(args...)
		loaded, ok := memo.Load(keys)
		if ok {
			if !Equals(actual, loaded) {
				panic("Do not tableize impure functions")
			}
		} else {
			memo.Store(keys, v)
		}
		return v
	}
}
```

---

## Novelty of Effect-ive Programming

- Focus on effects  
	- Emphasize effects as a goal, not just a means <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- Be yourself, respect your runtime  
	- Handle effects in a way that respects each language’s unique philosophy and idioms <!-- .element: style="font-size: 80%;" -->

---

## PS: Haskell vs Effect-ive Programming

- Haskell abstracts through mathematics  
	- A function is a pure mapping with one input and one output: output = f(input) <!-- .element: style="font-size: 80%;" -->
	- Programs are primarily composed through pure function composition<!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- Effects are essential in programs, but Haskell avoids executing them inside functions  
	- Thus, effects are only allowed in specific impure zones like `runX`, `main`, etc. <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- Effects must be deferred lazily → monads  
	- Encapsulate effects inside containers like `Either[T]` <!-- .element: style="font-size: 80%;" -->
	- How do you connect a pure input with an effectful output? <!-- .element: style="font-size: 80%;" -->
		- `f1: func(T1) Either[T2], f2: func(T2) Either[T3]` <!-- .element: style="font-size: 80%;" -->
	- Monad: an interface to connect containerized outputs with clean inputs <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- But problems arise when effects become deeply nested  
	- `Task[Either[T]] != Either[Task[T]]` <!-- .element: style="font-size: 80%;" -->
	- `StateReaderTaskEither[T]` <!-- .element: style="font-size: 80%;" -->

---

## PS: Haskell vs Effect-ive Programming

- Let’s approach programming more like humans do
	- People struggle with reasoning about deeply deferred and nested operations <!-- .element: style="font-size: 80%;" -->
	- Resolve effects eagerly instead <!-- .element: style="font-size: 80%;" -->
	- But I don’t want to perform them myself, so let’s delegate them now to someone else <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- Effects are not core logic, so they can be patterned and handled mechanically  
	- Assign handlers for each effect pattern and communicate with them <!-- .element: style="font-size: 80%;" -->
	- Where are those handlers? → Not DI, but IoC <!-- .element: style="font-size: 80%;margin-bottom: 1em;" -->
- We need a way to context-switch into a handler, perform the effect, and return  
	- Low-level abstraction: Continuation Passing Style <!-- .element: style="font-size: 80%;" -->
	- High-level abstraction: Communicating Sequential Processes <!-- .element: style="font-size: 80%;" -->
