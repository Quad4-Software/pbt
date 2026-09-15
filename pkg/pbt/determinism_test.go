// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"maps"
	"math/rand"
	"reflect"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func assertResultsIdentical[T comparable](t *testing.T, left, right pbt.Result[T]) {
	t.Helper()
	if left.Passed != right.Passed ||
		left.TimedOut != right.TimedOut ||
		left.CoverageFailed != right.CoverageFailed ||
		left.FailureIndex != right.FailureIndex ||
		left.Counterexample != right.Counterexample ||
		left.HasCounterexample != right.HasCounterexample {
		t.Fatalf("result fields diverged:\n%+v\n%+v", left, right)
	}
	if !reflect.DeepEqual(left.ShrinkTrace, right.ShrinkTrace) {
		t.Fatalf("shrink trace mismatch: %v vs %v", left.ShrinkTrace, right.ShrinkTrace)
	}
	if !reflect.DeepEqual(left.FailureLabels, right.FailureLabels) {
		t.Fatalf("failure labels mismatch: %v vs %v", left.FailureLabels, right.FailureLabels)
	}
	if !maps.Equal(left.LabelCounts, right.LabelCounts) {
		t.Fatalf("label counts mismatch: %v vs %v", left.LabelCounts, right.LabelCounts)
	}
	if !maps.Equal(left.BucketCounts, right.BucketCounts) {
		t.Fatalf("bucket counts mismatch: %v vs %v", left.BucketCounts, right.BucketCounts)
	}
	if !reflect.DeepEqual(left.CoverageErrors, right.CoverageErrors) {
		t.Fatalf("coverage errors mismatch: %v vs %v", left.CoverageErrors, right.CoverageErrors)
	}
	if left.Error() != right.Error() {
		t.Fatalf("error text mismatch: %q vs %q", left.Error(), right.Error())
	}
}

func TestSequentialDeterminismRepeatedRuns(t *testing.T) {
	property := pbt.ForAll(
		"deterministic sequential",
		pbt.IntRange(-300, 300),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
		pbt.WithLabeler(func(v int) []string {
			if v%2 == 0 {
				return []string{"even"}
			}
			return []string{"odd"}
		}),
		pbt.WithBucketer(func(v int) string {
			if v < 0 {
				return "neg"
			}
			return "nonneg"
		}),
		pbt.WithClassifier(func(v int) []string {
			if v > 0 {
				return []string{"positive"}
			}
			return []string{"nonpositive"}
		}),
	)

	opts := []pbt.Option{pbt.WithRuns(250), pbt.WithSeed(4242), pbt.WithMaxSize(50)}
	first := pbt.CheckResult(property, opts...)
	if first.Passed {
		t.Fatalf("expected failure so counterexample determinism is exercised")
	}
	for i := range 4 {
		next := pbt.CheckResult(property, opts...)
		assertResultsIdentical(t, first, next)
		_ = i
	}
}

func TestParallelDeterminismRepeatedRuns(t *testing.T) {
	property := pbt.ForAll(
		"deterministic parallel",
		pbt.IntRange(-300, 300),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
		pbt.WithLabeler(func(v int) []string {
			if v%2 == 0 {
				return []string{"even"}
			}
			return []string{"odd"}
		}),
		pbt.WithBucketer(func(v int) string {
			if v < 0 {
				return "neg"
			}
			return "nonneg"
		}),
	)

	for _, workers := range []int{2, 4, 7} {
		opts := []pbt.Option{
			pbt.WithRuns(300), pbt.WithSeed(777), pbt.WithMaxSize(40),
			pbt.WithParallelism(workers), pbt.WithShrinkParallelism(3),
		}
		first := pbt.CheckResult(property, opts...)
		if first.Passed {
			t.Fatalf("workers=%d: expected failure so determinism is exercised", workers)
		}
		for range 4 {
			next := pbt.CheckResult(property, opts...)
			assertResultsIdentical(t, first, next)
		}
	}
}

func TestParallelDeterminismOnPassingRun(t *testing.T) {
	property := pbt.ForAll(
		"deterministic parallel pass",
		pbt.IntRange(0, 100),
		func(v int) bool { return v >= 0 },
		pbt.WithLabeler(func(v int) []string {
			if v%2 == 0 {
				return []string{"even"}
			}
			return []string{"odd"}
		}),
	)

	opts := []pbt.Option{pbt.WithRuns(400), pbt.WithSeed(5), pbt.WithParallelism(6)}
	first := pbt.CheckResult(property, opts...)
	if !first.Passed {
		t.Fatalf("expected pass")
	}
	for range 4 {
		assertResultsIdentical(t, first, pbt.CheckResult(property, opts...))
	}
}

func TestStatefulDeterminismRepeatedRuns(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "det model",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			noopCommand{},
			addCommand{},
		},
		Invariant: func(state int) bool { return state < 3 },
	}

	assertSame := func(a, b pbt.StatefulResult[int]) {
		t.Helper()
		if a.Passed != b.Passed ||
			a.ScenarioSeed != b.ScenarioSeed ||
			a.ScenarioIndex != b.ScenarioIndex ||
			a.StepIndex != b.StepIndex ||
			a.FailedCommand != b.FailedCommand ||
			a.ShrinkPasses != b.ShrinkPasses ||
			a.FinalState != b.FinalState {
			t.Fatalf("stateful results diverged:\n%+v\n%+v", a, b)
		}
		if !reflect.DeepEqual(a.Trace, b.Trace) ||
			!reflect.DeepEqual(a.OriginalTrace, b.OriginalTrace) ||
			!reflect.DeepEqual(a.StepSeeds, b.StepSeeds) {
			t.Fatalf("stateful traces diverged")
		}
	}

	// Find a seed that actually shrinks so both shrink paths are exercised.
	var seed int64 = -1
	for s := int64(1); s <= 400; s++ {
		r := pbt.CheckStatefulResult(model, pbt.WithSeed(s), pbt.WithRuns(1), pbt.WithMaxSize(32))
		if !r.Passed && r.ShrinkPasses > 0 {
			seed = s
			break
		}
	}
	if seed < 0 {
		t.Fatalf("no shrinkable seed found")
	}

	for _, shrinkWorkers := range []int{1, 4} {
		opts := []pbt.Option{
			pbt.WithSeed(seed), pbt.WithRuns(1), pbt.WithMaxSize(32),
			pbt.WithShrinkParallelism(shrinkWorkers),
		}
		first := pbt.CheckStatefulResult(model, opts...)
		if first.Passed {
			t.Fatalf("expected failure")
		}
		for range 3 {
			assertSame(first, pbt.CheckStatefulResult(model, opts...))
		}
	}

	// Sequential and parallel shrink must reach the same minimal trace.
	seq := pbt.CheckStatefulResult(model, pbt.WithSeed(seed), pbt.WithRuns(1), pbt.WithMaxSize(32), pbt.WithShrinkParallelism(1))
	par := pbt.CheckStatefulResult(model, pbt.WithSeed(seed), pbt.WithRuns(1), pbt.WithMaxSize(32), pbt.WithShrinkParallelism(6))
	assertSame(seq, par)
}
