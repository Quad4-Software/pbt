// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestCoverageThresholdsFailPassingProperty(t *testing.T) {
	property := pbt.ForAll(
		"coverage thresholds",
		pbt.IntRange(1, 1),
		func(_ int) bool { return true },
		pbt.WithLabeler(func(_ int) []string { return []string{"present"} }),
		pbt.WithBucketer(func(_ int) string { return "bucket-present" }),
		pbt.WithLabelCoverageRules[int](pbt.CoverageRule{Key: "missing", MinCount: 1}),
		pbt.WithBucketCoverageRules[int](pbt.CoverageRule{Key: "bucket-missing", MinPercent: 1}),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(10), pbt.WithSeed(1))
	if result.Passed {
		t.Fatalf("expected coverage failure")
	}
	if !result.CoverageFailed {
		t.Fatalf("expected coverage failed flag")
	}
	if len(result.CoverageErrors) == 0 {
		t.Fatalf("expected coverage errors")
	}
}

func TestCheckFailsOnCoverageThresholds(t *testing.T) {
	property := pbt.ForAll(
		"check coverage failure",
		pbt.IntRange(1, 1),
		func(_ int) bool { return true },
		pbt.WithLabeler(func(_ int) []string { return []string{"a"} }),
		pbt.WithLabelCoverageRules[int](pbt.CoverageRule{Key: "b", MinCount: 1}),
	)

	testDouble := &fatalRecorder{}
	assertPanics(t, func() {
		pbt.Check(testDouble, property, pbt.WithRuns(5), pbt.WithSeed(11))
	})
	if !testDouble.called {
		t.Fatalf("expected fatal call from Check")
	}
}

type noopCommand struct{}

func (noopCommand) Name() string                     { return "noop" }
func (noopCommand) Precondition(_ int) bool          { return true }
func (noopCommand) Next(_ *rand.Rand, state int) int { return state }

type addCommand struct{}

func (addCommand) Name() string                     { return "add" }
func (addCommand) Precondition(_ int) bool          { return true }
func (addCommand) Next(_ *rand.Rand, state int) int { return state + 1 }

func TestStatefulCounterexampleShrinkingAndReplay(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "stateful shrink",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			noopCommand{},
			addCommand{},
		},
		Invariant: func(state int) bool {
			return state < 3
		},
	}

	var result pbt.StatefulResult[int]
	found := false
	for seed := int64(1); seed <= 200; seed++ {
		result = pbt.CheckStatefulResult(
			model,
			pbt.WithSeed(seed),
			pbt.WithRuns(1),
			pbt.WithMaxSize(64),
			pbt.WithShrinkParallelism(4),
		)
		if !result.Passed && len(result.OriginalTrace) > len(result.Trace) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected at least one seed to produce shrinkable stateful failure")
	}
	if result.ShrinkPasses == 0 {
		t.Fatalf("expected shrink passes to be recorded")
	}

	fixture, err := result.ToStatefulReplayFixture()
	if err != nil {
		t.Fatalf("unexpected fixture conversion error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "stateful-replay.json")
	if err := pbt.WriteStatefulReplayFixture(path, fixture); err != nil {
		t.Fatalf("unexpected fixture write error: %v", err)
	}

	replayed, err := pbt.ReplayStatefulFixtureFile(model, path)
	if err != nil {
		t.Fatalf("unexpected replay error: %v", err)
	}
	if replayed.Passed {
		t.Fatalf("expected replay to fail with same counterexample")
	}
}

func TestShrinkParallelismDoesNotChangeOutcome(t *testing.T) {
	property := pbt.ForAll(
		"parallel shrink tuning",
		pbt.IntRange(-200, 200),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	left := pbt.CheckResult(property, pbt.WithSeed(2026), pbt.WithRuns(300), pbt.WithShrinkParallelism(1))
	right := pbt.CheckResult(property, pbt.WithSeed(2026), pbt.WithRuns(300), pbt.WithShrinkParallelism(8))

	if left.Passed != right.Passed {
		t.Fatalf("pass mismatch for shrink parallelism")
	}
	if left.Counterexample != right.Counterexample {
		t.Fatalf("counterexample mismatch")
	}
}

func TestReplayFixtureRunner(t *testing.T) {
	property := pbt.ForAll(
		"replay property",
		pbt.IntRange(5, 5),
		func(v int) bool { return v == 0 },
	)
	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(77))
	if result.Passed {
		t.Fatalf("expected failing result")
	}
	fixture, err := result.ToReplayFixture()
	if err != nil {
		t.Fatalf("unexpected fixture conversion error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "replay.json")
	if err := pbt.WriteReplayFixture(path, fixture); err != nil {
		t.Fatalf("unexpected fixture write error: %v", err)
	}

	replayed, err := pbt.ReplayFixtureFile(property, path)
	if err != nil {
		t.Fatalf("unexpected replay error: %v", err)
	}
	if !replayed.Passed {
		t.Fatalf("replay operation should complete")
	}
	if !replayed.StillFailing {
		t.Fatalf("expected replayed value to still fail")
	}
	if !strings.Contains(replayed.Reason, "still fails") {
		t.Fatalf("unexpected replay reason: %q", replayed.Reason)
	}
}
