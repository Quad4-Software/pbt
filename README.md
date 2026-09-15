# pbt

Property-based testing for Go. A property pairs a generator with a predicate. The library runs the predicate against generated inputs, shrinks counterexamples, and records the seed so any failure replays exactly.

## Install

```bash
go get github.com/Quad4-Software/pbt@latest
```

For local development against a checkout, point a replace directive at it:

```go
replace github.com/Quad4-Software/pbt => ../pbt
```

```go
import "github.com/Quad4-Software/pbt/pkg/pbt"
```

## Quick start

```go
package mypkg_test

import (
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestStringRoundTrip(t *testing.T) {
	property := pbt.ForAll(
		"double reverse preserves value",
		pbt.StringASCII(0, 64),
		func(in string) bool {
			return reverse(reverse(in)) == in
		},
		pbt.WithShrinker[string](pbt.StringShrinker()),
	)

	pbt.Check(t, property, pbt.WithRuns(1000), pbt.WithSeed(42))
}
```

Outside the `testing` package, call `pbt.CheckResult` and inspect the returned result.

## API overview

- `ForAll` builds a property from a generator and a predicate. `ForAll2` and `ForAll3` take two or three generators and pass the values to the predicate directly. `Check` runs it and fails the test on the first counterexample. `CheckResult` runs it and returns a structured result.
- `WithPrecondition` rejects generated cases before the predicate runs, like QuickCheck implication. Rejected cases count in `Result.Skipped` and generation continues until `Runs` evaluated cases; if rejects exceed `MaxDiscards` the run fails with `Result.Exhausted`.
- `WithRuns`, `WithMaxSize`, `WithSeed`, `WithTimeout`, and `WithMaxDiscards` configure the run. `WithParallelism` splits seed partitions across workers. `WithShrinkParallelism` tunes shrink workers.
- Generators: `Int`, `IntRange`, `Int64`, `Int64Range`, `Uint64`, `Bool`, `Float64`, `StringASCII`, `String`, `Bytes`, `SliceOf`, `MapOf`, `Map`, `DurationRange`, `PtrOf`.
- Conditional generation: `SuchThat` and `SuchThatFallback` filter source values by predicate. `WithPrecondition` discards cases at the property level.
- Combinators: `Tuple2`, `Tuple3`, `Product2`, `OneOf`, `Frequency`, `FlatMap`, `Recursive`. `FlatMap` binds the output of one generator into the next, which builds correlated structures.
- Shrinkers: `IntShrinker`, `IntShrinkerToward`, `Int64Shrinker`, `Int64ShrinkerToward`, `Uint64Shrinker`, `StringShrinker`, `SliceShrinker`, `SliceShrinkerOf`, `BytesShrinker`, `Tuple2Shrinker`, `Tuple3Shrinker`. `SliceShrinkerOf` shrinks elements in place after removing what it can.
- `Sample` draws n values from a generator with a seed for inspecting distributions.
- Failure triage: `WithClassifier`, `WithLabeler`, `WithBucketer`. Coverage thresholds: `WithLabelCoverageRules`, `WithBucketCoverageRules`. Distribution checks: `AnalyzeDistribution`, `ValidateDistribution`.
- Stateful testing: `CheckStateful`, `CheckStatefulResult`, `CommandSequence`.
- Replay: `SerializeCounterexample`, `DeserializeCounterexample`, `Result.ToReplayFixture`, `ReplayFixtureFile`, `ReplayStatefulFixtureFile`.
- Hooks: `Hook`, `HookFuncs`, `WithHook`, `WithHooks`. Stateful hooks: `StatefulHook`, `StatefulHookFuncs`, `CommandModel.Hooks`.

## Reproducibility

Pass a seed with `WithSeed` to reproduce a failure exactly. The `Result` and the failure message both record the seed. A failing `Result` converts to a `ReplayFixture` for persisting to disk.

## Layout

- `pkg/pbt` holds the public API: generators, properties, shrinking, execution.
- `cmd/pbt-example` is a runnable example program.

## Taskfile

`Taskfile.yml` provides `task fmt`, `task vet`, `task lint`, `task test`, `task test:cov` (fails below 75% coverage), and `task ci`.

## License

0BSD. See [LICENSE](LICENSE).
