<p align="center">
  <img src="https://github.com/user-attachments/assets/bad6f14e-0444-4fbb-ad9d-74131c970568" />
  <br />
  <em>“The Zen of the Effect-ive Gopher” – calm, centered, and side-effect free.</em>
</p>


# What is Effect-ive Go?

**Effect-ive Go** is the first attempt to implement _Effect-ive Programming idiomatically in Go_. This project provides a **systematic way to isolate and handle side effects** on top of Go’s core principles—**goroutines**, **channels**, **context**, and **duck typing**.

* * *

# What is Effect-ive Programming?

**Effect-ive Programming** is an approach to building _predictable, testable, and reusable code_.

Its **first-class citizen** is the **Effect Pattern**: the idea is to **identify** impure parts of your logic (=side effects), and **isolate and delegate** them externally.

This is a **pattern-oriented approach**, not tied to any specific language or paradigm, making it applicable in any environment.

Effect-ive Programming generally follows three steps:

* 🔍 **Side effect recognition**: Identify non-pure logic
* 🧭 **Side effect isolation**: Isolate it into an external handler
* 📬 **Side effect delegation**: Delegate the effect to the handler

* * *

# Why does Effect-ive Programming matter? (TBD)

→ "Why does this matter?"→ Go-specific challenges → Solved via effect pattern → Real-world cases

> ✅ Before / After examples✅ Practical issues in Go: DI, error handling, config, context propagation, etc.

* * *

# What is Effect anyway?

### 💡 Pure Functions

* Executed **immediately upon call**
* **Deterministic**: Output depends only on input
* Do **nothing besides returning** the output (no side effects)

* * *

### ⚠️ Side Effects

* Everything that is **not a pure function**
* A helpful guide: the **4W Checklist** — if you violate even one, it’s a side effect:

| W   | Question |
| --- | --- |
| **Who** | Can someone else trigger this logic? |
| **What** | Does it mutate external state or return non-deterministic output? |
| **When** | Is it executed immediately upon call, or deferred? |
| **Where** | Does the behavior depend on where or under which context it's called? |

* * *

### 🔧 Effect Pattern

The purpose of the effect pattern is simple:

> 💬 _Delegate side effects to external handlers, so your logic remains predictable, testable, and reusable._

#### 🔍 1. Side Effect Recognition

* If even **one of the 4Ws is violated**, it qualifies as a side effect.
* But **not every side effect should be turned into a new effect**.

##### 🎯 Effect Atomicity Principle

> If a side effect can be composed using existing effects + pure functions, do **not** define a new effect.

* * *

#### 🧭 2. Side Effect Isolation

* Each effect is handled by a **dedicated effect handler**
* Every handler has a **clear scope**:
  * Only the closest handler is applied — **no implicit propagation**
  * Leaving the scope **automatically shuts down** the handler
  * Handlers **must not leak state** outside their scope
* **Effect scope should be close to where effects are performed**
  * Avoid placing all handlers at the composition root

* * *

#### 📬 3. Side Effect Delegation

Delegation is more than passing data — 👉 it’s about **transferring control flow ownership** to the handler.

##### ✳️ There are three types of delegation:

| Type | Description |
| --- | --- |
| **Resumable Effect** | Handler processes the effect and **returns the result** to the caller |
| **Fire-and-Forget** | Handler **executes without waiting**, and the caller resumes immediately |
| **Abortive Effect** | Handler processes and **aborts caller flow**, e.g., exceptions or panics |

* * *

## 📌 Summary

* An effect is any logic that **depends on context**, has **external interaction**, or violates **pure function guarantees**
* The **Effect Pattern** provides a structured way to **recognize, isolate, and delegate** them
* It forms a core foundation for **testable, modular, reusable architecture**

* * *

# How does Effect-ive Go work?

Effect-ive Go is designed around the idea of delegating side effects to dedicated **handlers**. Its core structure can be broken down into three main components:

* * *

## 1. 🧰 Effect Handler

Each handler is responsible for handling a specific type of side effect. All handlers satisfy the following conditions:

* **Scoped**: Handlers are only active within a specific context
* **Channel-based message handling**: Handlers run in goroutines and communicate through channels
* **Explicit lifecycle management**: Use `WithXxxEffectHandler(ctx)` to create and `defer cancel()` to release
* 🔒 **Not thread-safe by design**: Handlers are meant to be used only within a single goroutine to ensure proper scoping

* * *

## 2. 🧭 Effect Scope

Each handler is bound to a specific context.

* Within that context, the effect is **enabled**
* Once the context is exited, the handler is **automatically shut down**, and no effects can be handled

This guarantees:

* ❌ No handler leaks
* ❌ No upward propagation of effects (only **explicit delegation** is allowed)
* ✅ Handler lifecycle is **tracked via context**

* * *

## 3. 🔁 Effect Delegation

Effect-ive Go supports two main delegation models: resumable and fire-and-forget.

| Type | Description | Examples |
| --- | --- | --- |
| **Resumable Effect** | Processes the effect and returns the result | Reading config, state access |
| **Fire-and-Forget** | Executes asynchronously without waiting for a result | Logging, sending events |

→ Both models are implemented using **goroutines + channels**, and delegation always involves handing off **control flow ownership** at the point of `perform`.

* * *

### 🔍 Example Flow

    1. A handler scope is created with `WithStateEffectHandler(ctx)`
    2. Internally, a handler goroutine is spawned and waits on a `chan`
    3. Domain logic calls `PerformResumableEffect(ctx, EffectState, payload)`
    4. Handler processes the effect and sends result back on `resumeCh`
    5. Caller resumes execution with the result

* * *

## 4. 🧠 Declaring Effects Explicitly

Another important role of Effect-ive Programming is to explicitly surface all effects that occur within a given function.
In statically-typed languages with strong type inference, not only the kind of effects, but even their payload types can be reflected directly in function signatures.

In monadic systems, effects are expressed in the return type.
In handler-based systems, effects are reflected by the presence of handler types in the context or surrounding scope.

This means that every function, from the one that performs the effect to the one that handles it, must reveal the effect type in its signature.
However, Go lacks certain language features that make this kind of typing feasible:

❌ No sum types (tagged unions)
❌ No annotations or decorators to mark effect usage
❌ Function signatures can’t express anything beyond parameters and return values

✅ Effect-ive Go introduces two complementary strategies:

1️⃣ Effect-ive Lint (Static Analysis + Comment Injection)

A proposed tool will statically analyze the call stack from effect site to handler, and inject comments like this:

```go
// @effect: Log, State, Config
func MyServiceFunc(...) { ... }
```

At the composition root, if these comments remain, it means the handler for that effect is missing.

2️⃣ Die Loudly (Runtime Safety Net)

If an effect is performed without a handler in scope, the ideal behavior would be a compile-time failure.
Since Go doesn’t support this, Effect-ive Go panics at runtime instead.

This “die loudly” approach ensures that missing handlers are detected early during development and testing.

But caution:
In rarely executed branches, an unhandled effect might sneak into production.
That’s why careful code review and testing is essential when using dynamic effect systems in Go.


* * *

### 📌 Additional Design Notes

* Handlers are registered into context using **EffectEnum** as a key
* Hash partitioning is supported via the `Partitionable` interface
* Designed to be **panic-safe**, **explicitly terminated**, and **single-threaded** for maximum predictability

* * *

# How can I make custom effects? (TBD)

→ Guide for users to define their own effect handlers→ How to apply scope and implement custom logic

* * *

# Built-in Effects

| Effect | Type | Handler Responsibility |
| --- | --- | --- |
| 🧠 `State` | **Resumable** | Manage key-value state across goroutines<br>Sharded with `Partitionable` |
| 🔗 `Binding` | **Resumable** | Lookup config/flags/envs, with upper-scope fallback |
| 🧵 `Concurrency` | **Fire-and-Forget** | Spawn goroutines, gracefully terminate on cancel |
| 📝 `Log` | **Fire-and-Forget** | Async log emission via zap logger; ordering via `numWorkers = 1` |

* * *

# Effect-ive Programming vs Traditional Patterns

## 🧠 vs Functional Programming

* Shares the same goal as FP: isolating impurity for predictable, testable code
* Does not enforce everything to be pure functions chained by composition
* Focuses on **recognition and delegation** of effects through **Effect Pattern**, not the implementation style

* * *

## ⚙️ vs Monad-based Systems

* Monads chain effectful computations but introduce complexity
* Composing multiple effects (e.g., `Eff1[Eff2[T]]`) breaks intuition
* Algebraic Effect Handlers emerged to fix this
* **Effect-ive Programming** offers a **pragmatic and idiomatic** alternative

* * *

## 🧭 vs Effect Handler-Oriented Programming

| Aspect | Effect-ive Programming | EH-Oriented Programming |
| --- | --- | --- |
| **Goal** | Practical side effect isolation (SoC, testability, reuse) | Algebraic Effect abstraction at compiler level |
| **Handler Role** | Scope-bound isolation using goroutine/context/channel | Control flow rewiring via CPS |
| **Implementation** | Stays idiomatic (Go-specific) | Requires new features (CPS, algebraic effect) |
| **Design Limit** | Stays within language boundaries | Needs runtime/compiler support |
| **Recognition** | Based on 4W: Who, What, When, Where | Transformed effect contexts |
| **Use Cases** | Real-world concerns (log, state, cache, config, concurrency) | Language effects (exceptions, yield, fibers) |

* * *

## 🔌 vs Dependency Injection Frameworks

**Using effect systems like a DI container is a common anti-pattern.**

> 🛑 You should not return a service — just request an action and receive the result.

### ❌ Anti-pattern

    svc := PerformEffect(ctx, EffectDependency)
    return svc.DoSomething()

### ✅ Proper Usage

    result := PerformEffect(ctx, EffectDoSomething, input)
    return result

Effect systems are not about **injecting helpers** — they’re about **requesting side effects** and **receiving results**.

> 🧠 Don't “get something to do the job”—instead, “ask for the job to be done.”
