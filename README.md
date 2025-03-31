# Effect-ive Go

> *Algebraic Effect Handlers. Idiomatic Go. Functional clarity.*

* * *

## 🚧 Experimental Preview

> This project is still **experimental** and under active design. The API is subject to change.It’s being shared early to spark ideas, discussions, and contributions.

* * *

## ✨ What is Effect-ive Go?

**Effect-ive Go** brings **algebraic effect handlers** to the Go world—without monads, CPS, or complex abstractions.

Instead, it uses what Go gives us:

* goroutines
* channels
* context
* duck typing

The result: a system that **isolates side effects**, keeps your core logic **pure**, and supports highly **modular, testable, and reusable** code.

* * *

## 💡 Why Does This Matter? TODO 예문 변경

In any real-world Go application, you’ve probably seen this: 

    // Core logic tangled with logging, error handling, auth...
    if err := authCheck(user); err != nil {
        log.Println("unauthorized")
        return fmt.Errorf("...")
    }

With effect handlers, you can isolate these effects:

    // Core logic
    return FetchUser(id)

The actual logging, authorization, or retries happen *elsewhere*.They're **handled**, not **interwoven**.

* * *

## ⚠️ Design Philosophy: Effect ≠ Service Locator

One of the **biggest misconceptions** when using an effect system is treating it like a *service locator* or *DI container*. This is an anti-pattern.

> **Effect handlers should not return services or objects with behavior.**

For example, suppose you define a `DependencyEffect` that returns a service object, and then your domain logic calls a method on that service. At first glance, this may seem clean—but it's **worse** than simply injecting dependencies explicitly via parameters or struct fields.

Why?

Because now you've **hidden the dependency**, and the domain logic is no longer pure. You're pretending it's decoupled while still relying on an opaque service behind the scenes.

### ✅ What Should Happen Instead?

The **purpose** of performing an effect is *not* to receive a tool or method for later use.It is to **request that something be done elsewhere**, and to receive the **already completed result**.

    // ❌ Anti-pattern
    svc := PerformEffect(ctx, EffectDependency)
    return svc.DoSomething()
    
    // ✅ Proper usage
    result := PerformEffect(ctx, EffectDoSomething, input)
    return result

This ensures that the **core logic is completely separated** from how the effect is handled.

Effect systems are powerful because they let us **invert dependency direction** and enforce a clean **Separation of Concerns**. That only works if the **entire side effect is handled externally**, and domain logic **just receives the outcome**.

> 🧠 **Effect systems are not about "getting a thing to do a job"—they're about *asking* for the job to be done, and getting back the answer.**

* * *

## ⚠️ Avoid Global Handlers
> Are you planning to handle every error in your system the same way?
> Or put all your cached data into a single shared cache?
> Probably not.

Effect handlers are scoped.
And their power comes from locality—handling side effects close to where they happen.

Instead of defining a single, global handler at the composition root, ask:

- “What’s the best way to log right here?”
- “How should I handle errors in this function?”
- “Does this service need a retry logic that others don’t?”

Handlers can (and should) be redeclared at different depths.
That’s not duplication—it’s precision.

The nearest handler always wins.
That’s the point.

> Don’t treat effect handlers as global DI containers.
> Treat them as localized interpreters that make your code more expressive and intentional.


* * *

## 📦 Key Concepts

* **PerformEffect / WithEffect**: Register handlers and perform effects via `context`
* **Effect Enum**: Identify each effect type (e.g. `EffectLog`, `EffectConcurrency`)
* **Effect Scoping**: Handlers are scoped using `context.WithValue`, forming an implicit effect stack.
* **EffectStack**: Each goroutine owns its own stack. Handlers are resolved from nearest to root.
* **Teardown**: Each handler ensures end-of-scope cleanup. It must be idempotent.

* * *

## 🧪 Test Philosophy

Our tests aim to cover:

* Scoping and propagation
* Goroutine-level isolation
* Cancellation behavior
* Proper teardown invocation
* Panic safety
* Context-based resolution

Tests use **artificial effect types** for clarity.Real-world examples come in `/examples`.

* * *

## 🧰 Status

| Feature | Status |
| --- | --- |
| 🔹 `WithEffect`, `PerformEffect` | ✅ Stable |
| 🔸 `ConcurrencyEffect` (goroutine orchestration) | ✅ Experimental |
| 🔸 Test coverage | ⏳ Ongoing |
| 🔸 Examples | ⏳ Planned |
| 🔸 Cancellable resource effects | 🧪 Under review |

* * *

## 📎 How This Is Different

* **No monads**.
* **No continuation-passing style**.
* **No macros or code-gen**.

Only Go. Only idioms you already know.

    ctx := context.WithValue(ctx, EffectLog, myHandler)
    PerformEffect(ctx, EffectLog, "logging this event")


* * *

## 📬 Contributing

Want to help?

* Raise issues or ideas.
* Share your use case.
* Or just play with it and tell us what’s missing.

Together, we can shape a new way of writing Go.

* * *

## 🙏 Acknowledgements

Built on the insights of:

* [Arrow-KT](https://arrow-kt.io/)
* [Ocaml5](https://ocaml.org/manual/5.3/effects.html)
* [Go's beautiful simplicity]

And the realization that **purity doesn't need to be painful.**

* * *

## 🕊 License

MIT
