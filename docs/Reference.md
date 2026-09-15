# pbt Reference

Property-based testing library for Go.

**Module:** `github.com/Quad4-Software/pbt`  
**Import:** `github.com/Quad4-Software/pbt/pkg/pbt`  
**Go:** 1.24+

---

## Core Flow

1. Define a property with `ForAll(name, generator, predicate, opts...)`
2. Run with `Check(t, property, opts...)` (testing) or `CheckResult(property, opts...)` (programmatic)
3. On failure: `Result` contains `Counterexample`, `Seed`, `ShrinkTrace`; use `WithSeed(seed)` to reproduce

---

## Execution

### Check

```go
func Check[T any](t TestingT, property Property[T], opts ...Option)
```

Runs the property and calls `t.Fatalf` on failure. Use from `*testing.T`.

### CheckResult

```go
func CheckResult[T any](property Property[T], opts ...Option) Result[T]
```

Runs the property and returns a structured result. Inspect `result.Passed`, `result.Counterexample`, `result.Seed`.

### TestingT

```go
type TestingT interface {
    Helper()
    Fatalf(format string, args ...any)
}
```

Subset of `testing.TB` required by `Check`. Satisfied by `*testing.T`, `*testing.B`.

---

## Property Definition

### ForAll

```go
func ForAll[T any](name string, generator Generator[T], predicate Predicate[T], opts ...PropertyOption[T]) Property[T]
```

Constructs a property. `predicate` must return `true` for the property to hold.

### Property

```go
type Property[T any] struct {
    Name       string
    Generator  Generator[T]
    Predicate  Predicate[T]
    Shrinker   Shrinker[T]     // optional
    Classifier Classifier[T]    // optional
    Labeler    Labeler[T]      // optional
    Bucketer   Bucketer[T]     // optional
    Coverage   CoverageConfig  // optional
    Hooks      []Hook[T]       // optional
}
```

### Predicate

```go
type Predicate[T any] func(value T) bool
```

Returns `true` when the property holds for the given value.

---

## Execution Options (Option)

| Option | Effect |
|-------|--------|
| `WithRuns(n)` | Number of generated cases (default 100); panics if n <= 0 |
| `WithMaxSize(n)` | Max size param to generators (default 100) |
| `WithSeed(seed)` | Deterministic seed; required for reproducibility |
| `WithTimeout(d)` | Abort after duration; 0 = no limit |
| `WithParallelism(n)` | Worker count; deterministic partitioning |
| `WithShrinkParallelism(n)` | Workers for shrink minimization |

---

## Generators

### Interface

```go
type Generator[T any] interface {
    Generate(r *rand.Rand, size int) T
    Name() string
}
```

### Built-in

| Function | Type | Description |
|---------|------|-------------|
| `Int()` | `Generator[int]` | Full int range |
| `IntRange(low, high)` | `Generator[int]` | Inclusive [low, high] |
| `Bool()` | `Generator[bool]` | Random bool |
| `Float64()` | `Generator[float64]` | [0, 1) |
| `StringASCII(low, high)` | `Generator[string]` | ASCII alphanumeric, length in [low, high] |
| `SliceOf(elem, low, high)` | `Generator[[]T]` | Slice of elem-generated values |
| `Map(name, source, mapper)` | `Generator[B]` | Transform A to B |

### SuchThat

```go
func SuchThat[T any](name string, source Generator[T], predicate Predicate[T], maxAttempts int) Generator[T]
```

Filters generated values to those satisfying the predicate. Tries up to maxAttempts; panics if none found. Use maxAttempts <= 0 for default (1000).

### SuchThatFallback

```go
func SuchThatFallback[T any](name string, source Generator[T], predicate Predicate[T], fallback T, maxAttempts int) Generator[T]
```

Like SuchThat but returns fallback when exhausted instead of panicking. Fallback must satisfy the predicate.

### Combinators

| Function | Description |
|---------|-------------|
| `Tuple2(name, a, b)` | Product generator; yields `Tuple2Value[A,B]{First, Second}` |
| `Tuple3(name, a, b, c)` | Triple product |
| `Product2` | Alias for Tuple2 |
| `OneOf(name, gens...)` | Pick one generator uniformly |
| `Frequency(name, entries...)` | Weighted choice; `entries` are `WeightedGenerator[T]{Weight, Generator}` |
| `Recursive(name, base, combine, maxDepth)` | Recursive structure; `combine(self) -> Generator[T]` |

### Custom Generator

```go
gen := pbt.NewGenerator("mygen", func(r *rand.Rand, size int) MyType {
    return MyType{...}
})
```

---

## Shrinkers

### Interface

```go
type Shrinker[T any] interface {
    Shrink(value T, predicate Predicate[T]) (T, bool)
}
```

Returns (shrunk value, true if changed). Optional extensions: `TraceShrinker`, `ParallelTraceShrinker`.

### Built-in

| Function | Type | Strategy |
|---------|------|----------|
| `IntShrinker()` | `Shrinker[int]` | Halve toward zero |
| `StringShrinker()` | `Shrinker[string]` | Halve length, trim space |
| `SliceShrinker[T]()` | `Shrinker[[]T]` | Remove elements; try halves, then one-by-one; implements TraceShrinker, ParallelTraceShrinker |

### Custom Shrinker

```go
shrinker := pbt.ShrinkerFunc[MyType](func(v MyType, pred pbt.Predicate[MyType]) (MyType, bool) {
    // return smaller v that still fails pred, or (v, false) if cannot shrink
    return shrunk, changed
})
```

---

## Property Options (PropertyOption)

| Option | Effect |
|--------|--------|
| `WithShrinker(shrinker)` | Minimize counterexamples |
| `WithClassifier(fn)` | Tag failures; `func(T) []string` |
| `WithLabeler(fn)` | Label each sample; `func(T) []string` |
| `WithBucketer(fn)` | Assign to bucket; `func(T) string` |
| `WithLabelCoverageRules(rules...)` | Enforce label thresholds |
| `WithBucketCoverageRules(rules...)` | Enforce bucket thresholds |
| `WithHook(hook)` | Lifecycle callbacks |
| `WithHooks(hooks...)` | Multiple hooks |

### CoverageRule

```go
type CoverageRule struct {
    Key        string  // Label or bucket id
    MinCount   int     // Minimum absolute count
    MinPercent float64 // Minimum percentage 0-100
}
```

---

## Result

```go
type Result[T any] struct {
    Passed            bool
    TimedOut          bool
    CoverageFailed    bool
    PropertyName      string
    Runs              int
    Seed              int64
    GeneratorName     string
    Counterexample    T
    HasCounterexample bool
    FailureLabels     []string
    ShrinkTrace       []T
    LabelCounts       map[string]int
    BucketCounts      map[string]int
    CoverageErrors    []string
}
```

`Error() string` returns a readable failure message including seed and counterexample.

---

## Replay Fixtures

Persist and replay failing cases.

### Stateless

```go
fixture, _ := result.ToReplayFixture()
pbt.WriteReplayFixture("fail.json", fixture)

fixture, _ = pbt.ReadReplayFixture("fail.json")
replay, _ := pbt.ReplayFixtureValue(property, fixture)
// or: pbt.ReplayFixtureFile(property, "fail.json")
```

### Stateful

```go
fixture, _ := result.ToStatefulReplayFixture()
pbt.WriteStatefulReplayFixture("fail.json", fixture)

fixture, _ = pbt.ReadStatefulReplayFixture("fail.json")
replay := pbt.ReplayStatefulFixture(model, fixture)
// or: pbt.ReplayStatefulFixtureFile(model, "fail.json")
```

### Serialization

```go
pbt.SerializeCounterexample[T](value)   // -> (string, error)
pbt.DeserializeCounterexample[T](json) // -> (T, error)
```

---

## Stateful (Model-Based) Testing

### CommandModel

```go
type CommandModel[S any] struct {
    Name      string
    Init      func(r *rand.Rand) S
    Commands  []StatefulCommand[S]
    Invariant func(state S) bool
    Hooks     []StatefulHook[S]
}
```

### StatefulCommand

```go
type StatefulCommand[S any] interface {
    Name() string
    Precondition(state S) bool
    Next(r *rand.Rand, state S) S
}
```

### Execution

```go
pbt.CheckStateful(t, model, pbt.WithSeed(42), pbt.WithRuns(100))
// or
result := pbt.CheckStatefulResult(model, opts...)
```

### CommandSequence

```go
gen := pbt.CommandSequence("seq", minLen, maxLen, cmd1, cmd2, ...)
```

Generates random command sequences of length [minLen, maxLen].

### StatefulResult

```go
type StatefulResult[S any] struct {
    Passed        bool
    ModelName     string
    Seed          int64
    ScenarioSeed   int64
    ScenarioIndex int
    StepIndex     int
    FailedCommand string
    Trace         []string
    StepSeeds     []int64
    FinalState    S
    // ...
}
```

---

## Distribution

### AnalyzeDistribution

```go
func AnalyzeDistribution[T any](
    generator Generator[T],
    bucketer func(T) string,
    runs int, seed int64, maxSize int,
) DistributionReport
```

### ValidateDistribution

```go
func ValidateDistribution(report DistributionReport, rules []DistributionRule) error
```

### DistributionRule

```go
type DistributionRule struct {
    Bucket     string
    MinPercent float64
    MaxPercent float64
}
```

---

## Hooks

### Hook Interface

```go
type Hook[T any] interface {
    OnRunStart(event RunStartEvent)
    OnCaseGenerated(event CaseGeneratedEvent[T])
    OnFailure(event FailureEvent[T])
    OnShrinkStep(event ShrinkStepEvent[T])
    OnRunEnd(event RunEndEvent)
    OnReplay(event ReplayEvent[T])
}
```

### HookFuncs

```go
pbt.HookFuncs[T]{
    RunStart:      func(e RunStartEvent) { ... },
    CaseGenerated: func(e CaseGeneratedEvent[T]) { ... },
    Failure:       func(e FailureEvent[T]) { ... },
    ShrinkStep:    func(e ShrinkStepEvent[T]) { ... },
    RunEnd:        func(e RunEndEvent) { ... },
    Replay:        func(e ReplayEvent[T]) { ... },
}
```

Nil fields are ignored.

---

## Minimal Example

```go
package mypkg_test

import (
    "testing"
    "github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestDoubleReverse(t *testing.T) {
    pbt.Check(t,
        pbt.ForAll(
            "double reverse preserves value",
            pbt.StringASCII(0, 64),
            func(s string) bool { return reverse(reverse(s)) == s },
            pbt.WithShrinker[string](pbt.StringShrinker()),
        ),
        pbt.WithRuns(1000),
        pbt.WithSeed(42),
    )
}
```

---

## Reproducibility

- Default config uses seed 0 for reproducibility. Use `WithSeed(seed)` to override.
- `Result.Seed` and `Result.Error()` include the seed on failure.
- Persist via `ToReplayFixture` / `ToStatefulReplayFixture` and replay later.

---

## Taskfile Commands

| Task | Description |
|------|-------------|
| `task test` | Run tests |
| `task test:leak` | Leak-focused tests |
| `task test:fuzz` | Fuzz targets (bounded) |
| `task test:all` | test + leak + fuzz |
| `task test:cov` | Coverage; fails if < 75% |
| `task fmt` | Format |
| `task vet` | go vet |
| `task lint` | revive |
| `task ci` | fmt, vet, lint, test:all |
