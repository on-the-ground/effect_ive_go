<p align="center">
  <img src="https://github.com/user-attachments/assets/bad6f14e-0444-4fbb-ad9d-74131c970568" />
  <br />
  <em>“The Zen of the Effect-ive Gopher” – calm, centered, and side-effect free.</em>
</p>

# The [presentation](https://on-the-ground.github.io/effect_ive_go/#/) is ready!

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

## Why does Effect-ive Programming matter?

In real-world applications, side effects are everywhere—logging, reading config, managing shared state, spawning goroutines, or interacting with the network. These effects make our code:

- harder to **test**
    
- difficult to **reason about**
    
- fragile to **reuse or compose**
    

Effect-ive Programming offers a systematic way to isolate and delegate those side effects, so your business logic remains:

- **predictable** — no hidden interactions or surprises
    
- **testable** — you can mock or replace effects freely
    
- **reusable** — pure logic is portable across contexts
    

By explicitly recognizing effects and pushing them to the boundary of your application, you enforce **separation of concerns** and avoid coupling your core logic to runtime behavior.

Even in languages like Go—without native effect systems—you can regain this structure through simple conventions and disciplined handler design.  
This is what **Effect-ive Go** makes possible.

* * *

> ✅ Before / After examples✅ Practical issues in Go: DI, error handling, config, context propagation, etc.(TBD)

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

### 🧭 Categorizing Effects by the 4W Criteria

> ✅ An effect is anything that violates **Who**, **When**, **Where**, or **What**.

An effect arises when a function violates one or more of the following 4W guarantees. Each type of violation leads to a specific class of side effect.


|4W Criteria|Effect Type|Description|Examples|
|---|---|---|---|
|**Who**: Control Flow Ownership“Is someone else executing this?”|**Concurrency Effect**|Delegates execution to another unit of control flow (thread, goroutine, etc.)|`go func() { ... }`|
|**When**: Execution Timing Guarantee“Is execution guaranteed immediately?”|**Task Effect**|Execution may happen **later**, depending on the runtime or external trigger|`http.Get(...)`|
|**Where**: Context Awareness“Which context is this running in?”|**Binding / State Effect**|Behavior depends on the current scoped context or environment|context.WithValue()</br>request.Context()</br>ThreadLocal</br>this(JavaScript)|
|**What**: Predictable Result“Does the same input always produce the same output?”|**Time / Random / IO Effect**|Depends on or mutates internal/external state|rand.Intn(n)</br>time.Now()</br>globalCounter++</br>db.Save()|


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
| **Resumable Effect** | The handler processes the effect and returns the result to the initiator, who waits for the handling to complete before resuming. |
| **Fire-and-Forget** | The initiator triggers the effect and resumes immediately, without waiting for the handler to finish processing. |
| **Abortive Effect** | The handler processes the effect and terminates the initiator's flow, e.g., by raising a panic or error that escapes the current scope. |

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
    3. Domain logic calls `PerformResumableEff(ctx, EffectState, payload)`
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

* ❌ No sum types (tagged unions)
* ❌ No annotations or decorators to mark effect usage
* ❌ Function signatures can’t express anything beyond parameters and return values

✅ Effect-ive Go introduces two complementary strategies:

### 1️⃣ Effect-ive Lint (Static Analysis + Comment Injection)

A proposed tool will statically analyze the call stack from effect site to handler, and inject comments like this:

```go
// @effect: Log, State, Config
func MyServiceFunc(...) { ... }
```

At the composition root, if these comments remain, it means the handler for that effect is missing.

### 2️⃣ Die Loudly (Runtime Safety Net)

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


## ✍️ Custom Effect Design Tips

While building your own custom effects, especially `ResumableEffect`, keep the following principle in mind:

> ⚠️ **Avoid invoking resumable effects _inside_ other effect handlers.**

### Why?

Resumable effects involve not just side-effect execution but also **waiting for a response**. If a handler itself triggers another resumable effect, you introduce:

- ❗ **Coupling between effects**
    
- ❗ **Non-determinism** in handler behavior
    
- ❗ **Hidden control flow delegation**, which breaks testability and predictability
    

### 🔄 Analogy

This is similar to avoiding "nested monads" in functional programming. Instead of:

```go
// ❌ Don't do this inside an effect handler 
msg.ResumeCh <- PerformResumableEff(ctx, EffectConfig, payload)
```

Do this instead:

```go
// ✅ Do it outside and pass as explicit input 
configValue := PerformResumableEff(ctx, EffectConfig, payload)  PerformResumableEff(ctx, MyEffect, configValue)
```

### 🧘‍♂️ Principle

> Keep your effects **flat**.  
> Handlers should **not rely on other handlers** to work correctly.

Let the **caller** take control of effect composition:

- Scope the config effect handler first
    
- Query the value
    
- Then pass it to the other effect handler as an input
    

### 💡 Exception?

**Fire-and-Forget** effects (e.g., logging) are safe to call inside other handlers.

```go
// ✅ This is fine inside any handler 
LogEff(ctx, LogInfo, "processing payload", map[string]any{"key": payload.Key})
```

Because they **do not block** and **do not depend on return values**, they’re harmless and often useful for internal visibility.


* * *

# Built-in Effects

| Effect | Type | Handler Responsibility |
| --- | --- | --- |
| 🧠 `State` | **Resumable** | Manage key-value state across goroutines<br>Sharded with `Partitionable` |
| 🔗 `Binding` | **Resumable** | Lookup config/flags/envs, with upper-scope fallback |
| 🧵 `Concurrency` | **Fire-and-Forget** | Spawn goroutines, gracefully terminate on cancel |
| 📝 `Log` | **Fire-and-Forget** | Async log emission via zap logger; ordering via `numWorkers = 1` |

* * *
# 🛠 Real-World Refactoring Examples

## 1. Configuration Lookup (Binding Effect)

### 🔴 Before

    apiKey := os.Getenv("API_KEY")
    if apiKey == "" {
        log.Fatal("missing API_KEY")
    }

### ✅ After (Effect-ive)

    apiKey, err := GetFromBindingEffect[string](ctx, "API_KEY")
    if err != nil {
        LogEff(ctx, LogError, "missing binding", map[string]any{"key": "API_KEY"})
        return err
    }

* * *

## 2. Logging (Log Effect)

### 🔴 Before

    log.Printf("Processing user %s", userID)

### ✅ After (Effect-ive)

    LogEff(ctx, LogInfo, "processing user", map[string]any{"userID": userID})

* * *

## 3. State Management (State Effect)

### 🔴 Before

    cache := make(map[string]any)
    cache["session"] = sessionData

### ✅ After (Effect-ive)

    StateEff(ctx, SetStatePayload{Key: "session", Value: sessionData})

* * *

## 4. Spawning Goroutines (Concurrency Effect)

### 🔴 Before

    go func() {
        process(userID)
    }()

### ✅ After (Effect-ive)

    ConcurrencyEff(ctx, []func(context.Context){
        func(ctx context.Context) { process(userID) },
    })

* * *

## 🤔 Why is there no `RaiseEffect`?

Some languages treat exceptions or errors as **effects**—an unexpected break from the normal execution flow. But Go's approach is fundamentally different, which is why Effect-ive Go does **not** provide a built-in `RaiseEffect`.

### Here's why:

- ✅ **In Go, errors are values.**
    
    Errors are part of the regular return values (`val, err := ...`). They're not magical, and they don’t break flow like exceptions.
    
- ✅ **Go supports multiple return values.**
    
    Unlike most languages (Java, Python, etc.) that return only a single value, Go can return a result and an error side-by-side. This removes the need for an external `RaiseEffect`.
    
- ✅ **Error handling is not an effect in Go's model.**
    
    Effects typically represent **external interactions** or **non-determinism** (e.g., time, I/O, logging). In contrast, Go treats errors as part of deterministic flow: they propagate up as values.
    
- ✅ **Errors in Go are not "events".**
    
    They’re not thrown and caught like in other languages. They're wrapped, enriched with context, and passed up the call chain explicitly.
    

So, while effect systems in other languages (like Kotlin or OCaml 5) must model error propagation as an effect, Go's idioms make this unnecessary.

> 📌 If your app is raising exceptions as control flow, reconsider. In Go, **explicit error returns** remain idiomatic—and Effect-ive Go encourages this clarity.

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

    svc := PerformEff(ctx, EffectDependency)
    return svc.DoSomething()

### ✅ Proper Usage

    result := PerformEff(ctx, EffectDoSomething, input)
    return result

Effect systems are not about **injecting helpers** — they’re about **requesting side effects** and **receiving results**.

> 🧠 Don't “get something to do the job”—instead, “ask for the job to be done.”
* * *
> 🧘‍♂️ Effect-ive Go follows “[The Zen of Go](https://the-zen-of-go.netlify.app/)”
> Designed with simplicity, clarity, and maintainability at heart.
> From scope lifecycle to panic safety, every decision is deliberate.
> Because Go deserves idiomatic effect handling — with Zen.
* * *
