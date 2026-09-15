// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestRunsOneExecutesExactlyOnce(t *testing.T) {
	var events []pbt.CaseGeneratedEvent[int]
	hook := pbt.HookFuncs[int]{
		CaseGenerated: func(e pbt.CaseGeneratedEvent[int]) {
			events = append(events, e)
		},
	}
	property := pbt.ForAll(
		"single run",
		pbt.IntRange(0, 100),
		func(_ int) bool { return true },
		pbt.WithHook[int](hook),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithMaxSize(33), pbt.WithSeed(2))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if len(events) != 1 {
		t.Fatalf("expected exactly one case event, got %d", len(events))
	}
	if events[0].Index != 0 {
		t.Fatalf("unexpected case index: %d", events[0].Index)
	}
	if events[0].Size != 33 {
		t.Fatalf("single run must use MaxSize, got %d", events[0].Size)
	}
}

func TestMaxSizeOneAlwaysUsesSizeOne(t *testing.T) {
	var sizes []int
	generator := pbt.NewGenerator("probe", func(_ *rand.Rand, size int) int {
		sizes = append(sizes, size)
		return 0
	})
	property := pbt.ForAll("size one", generator, func(_ int) bool { return true })

	result := pbt.CheckResult(property, pbt.WithRuns(25), pbt.WithMaxSize(1), pbt.WithSeed(4))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	for i, s := range sizes {
		if s != 1 {
			t.Fatalf("run %d used size %d, expected 1", i, s)
		}
	}
}

func TestParallelismExceedsRuns(t *testing.T) {
	property := pbt.ForAll(
		"more workers than runs",
		pbt.IntRange(0, 3),
		func(_ int) bool { return false },
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	result := pbt.CheckResult(
		property,
		pbt.WithRuns(5),
		pbt.WithParallelism(32),
		pbt.WithSeed(8),
	)
	if result.Passed {
		t.Fatalf("expected failure on the very first case")
	}
	if result.FailureIndex != 0 {
		t.Fatalf("expected failure at index 0, got %d", result.FailureIndex)
	}
	// With an always-false predicate the int shrinker halves to zero.
	if !result.HasCounterexample || result.Counterexample != 0 {
		t.Fatalf("unexpected counterexample: %v", result.Counterexample)
	}
}

func TestIntRangeSinglePointIsConstant(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	gen := pbt.IntRange(7, 7)
	for range 50 {
		if v := gen.Generate(rng, 10); v != 7 {
			t.Fatalf("expected constant 7, got %d", v)
		}
	}
}

func TestStringASCIINegativeAndSwappedBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(4))

	allNegative := pbt.StringASCII(-5, -1)
	for range 50 {
		if v := allNegative.Generate(rng, 10); len(v) != 0 {
			t.Fatalf("expected empty string for negative bounds, got %q", v)
		}
	}

	swapped := pbt.StringASCII(5, 2)
	for range 50 {
		if v := swapped.Generate(rng, 10); len(v) < 2 || len(v) > 5 {
			t.Fatalf("swapped bounds not normalized: %q", v)
		}
	}
}

func TestSliceOfNegativeBoundsProducesEmpty(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	gen := pbt.SliceOf(pbt.IntRange(0, 9), -4, -1)
	for range 20 {
		if v := gen.Generate(rng, 10); len(v) != 0 {
			t.Fatalf("expected empty slice for negative bounds, got %v", v)
		}
	}
}

func TestCommandSequenceNegativeBoundsRegression(t *testing.T) {
	// minLen and maxLen both negative must clamp to zero, not panic with a
	// negative slice capacity.
	gen := pbt.CommandSequence("neg", -5, -1, bankModel{})
	rng := rand.New(rand.NewSource(6))
	for range 20 {
		if seq := gen.Generate(rng, 5); len(seq) != 0 {
			t.Fatalf("expected empty sequence, got %d commands", len(seq))
		}
	}
}

func TestNilHookIsSkipped(t *testing.T) {
	property := pbt.ForAll(
		"nil hooks",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
		pbt.WithHook[int](nil),
		pbt.WithHooks[int](nil, nil),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(5), pbt.WithSeed(1))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
}

func TestCoverageRuleWithoutThresholdPanics(t *testing.T) {
	property := pbt.ForAll(
		"vacuous rule",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
		pbt.WithLabelCoverageRules[int](pbt.CoverageRule{Key: "x"}),
	)
	assertPanics(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(3), pbt.WithSeed(1))
	})

	bucketed := pbt.ForAll(
		"vacuous bucket rule",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
		pbt.WithBucketCoverageRules[int](pbt.CoverageRule{Key: "y"}),
	)
	assertPanics(t, func() {
		_ = pbt.CheckResult(bucketed, pbt.WithRuns(3), pbt.WithSeed(1))
	})
}

func TestCoverageRuleValidationPanics(t *testing.T) {
	base := func() pbt.Property[int] {
		return pbt.ForAll("v", pbt.IntRange(0, 1), func(_ int) bool { return true })
	}

	cases := []pbt.Property[int]{
		func() pbt.Property[int] {
			p := base()
			p.Coverage.LabelRules = []pbt.CoverageRule{{Key: "", MinCount: 1}}
			return p
		}(),
		func() pbt.Property[int] {
			p := base()
			p.Coverage.LabelRules = []pbt.CoverageRule{{Key: "a", MinCount: -1}}
			return p
		}(),
		func() pbt.Property[int] {
			p := base()
			p.Coverage.BucketRules = []pbt.CoverageRule{{Key: "a", MinPercent: 101}}
			return p
		}(),
		func() pbt.Property[int] {
			p := base()
			p.Coverage.BucketRules = []pbt.CoverageRule{{Key: "a", MinPercent: -1}}
			return p
		}(),
	}
	for i, property := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("case %d: expected panic", i)
				}
			}()
			_ = pbt.CheckResult(property, pbt.WithRuns(3), pbt.WithSeed(1))
		}()
	}
}

func TestEmptyLabelsAndBucketsAreSkipped(t *testing.T) {
	property := pbt.ForAll(
		"empty coverage keys",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
		pbt.WithLabeler(func(_ int) []string { return []string{"", "x", ""} }),
		pbt.WithBucketer(func(_ int) string { return "" }),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(10), pbt.WithSeed(2))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if len(result.LabelCounts) != 1 || result.LabelCounts["x"] != 10 {
		t.Fatalf("unexpected label counts: %v", result.LabelCounts)
	}
	if len(result.BucketCounts) != 0 {
		t.Fatalf("empty bucket must not be counted: %v", result.BucketCounts)
	}
}

func TestPassingRunCountsCoverAllRuns(t *testing.T) {
	property := pbt.ForAll(
		"full coverage",
		pbt.IntRange(0, 9),
		func(_ int) bool { return true },
		pbt.WithLabeler(func(v int) []string {
			if v%2 == 0 {
				return []string{"even"}
			}
			return []string{"odd"}
		}),
		pbt.WithBucketer(func(v int) string {
			if v < 5 {
				return "low"
			}
			return "high"
		}),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(37), pbt.WithSeed(9))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	total := 0
	for _, c := range result.LabelCounts {
		total += c
	}
	if total != 37 {
		t.Fatalf("label counts must cover all runs: %d", total)
	}
	total = 0
	for _, c := range result.BucketCounts {
		total += c
	}
	if total != 37 {
		t.Fatalf("bucket counts must cover all runs: %d", total)
	}
}

func TestCustomOptionBypassStillValidated(t *testing.T) {
	property := pbt.ForAll("x", pbt.IntRange(0, 1), func(_ int) bool { return true })
	sneaky := pbt.Option(func(c *pbt.Config) { c.Runs = -5 })
	assertPanics(t, func() {
		_ = pbt.CheckResult(property, sneaky)
	})
}

func TestOneOfEmptyPanicsOnGenerate(t *testing.T) {
	gen := pbt.OneOf[int]("empty")
	rng := rand.New(rand.NewSource(1))
	assertPanics(t, func() {
		gen.Generate(rng, 5)
	})
}

func TestFrequencyPanicsOnInvalidWeights(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	empty := pbt.Frequency[int]("empty")
	assertPanics(t, func() { empty.Generate(rng, 5) })

	zero := pbt.Frequency[int]("zero",
		pbt.WeightedGenerator[int]{Weight: 0, Generator: pbt.IntRange(0, 1)},
		pbt.WeightedGenerator[int]{Weight: -2, Generator: pbt.IntRange(0, 1)},
	)
	assertPanics(t, func() { zero.Generate(rng, 5) })
}

func TestFrequencySkipsNonPositiveWeights(t *testing.T) {
	rng := rand.New(rand.NewSource(12))
	gen := pbt.Frequency("skip",
		pbt.WeightedGenerator[int]{Weight: 0, Generator: pbt.IntRange(0, 0)},
		pbt.WeightedGenerator[int]{Weight: -1, Generator: pbt.IntRange(1, 1)},
		pbt.WeightedGenerator[int]{Weight: 1, Generator: pbt.IntRange(2, 2)},
	)
	for range 100 {
		if v := gen.Generate(rng, 5); v != 2 {
			t.Fatalf("non-positive weight entry was selected: %d", v)
		}
	}
}

func TestSuchThatDefaultAttempts(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	gen := pbt.SuchThat("default attempts", pbt.IntRange(0, 9), func(v int) bool { return v == 5 }, 0)
	v := gen.Generate(rng, 10)
	if v != 5 {
		t.Fatalf("expected 5, got %d", v)
	}
}

func TestRecursiveGeneratorRespectsDepthBound(t *testing.T) {
	base := pbt.NewGenerator("leaf", func(_ *rand.Rand, _ int) string { return "leaf" })
	combine := func(self pbt.Generator[string]) pbt.Generator[string] {
		return pbt.Map("node", self, func(s string) string { return "node(" + s + ")" })
	}
	gen := pbt.Recursive("tree", base, combine, 4)

	rng := rand.New(rand.NewSource(33))
	sawNested := false
	for range 300 {
		v := gen.Generate(rng, 100)
		depth := strings.Count(v, "node(")
		if depth > 4 {
			t.Fatalf("recursion depth exceeded bound: %q", v)
		}
		sawNested = sawNested || depth > 0
	}
	if !sawNested {
		t.Fatalf("recursive generator never produced a nested value")
	}
}

func TestMapPropagatesName(t *testing.T) {
	mapped := pbt.Map("doubled", pbt.IntRange(0, 1), func(v int) int { return v * 2 })
	if mapped.Name() != "doubled" {
		t.Fatalf("expected generator name doubled, got %q", mapped.Name())
	}
}

func TestAnalyzeDistributionValidation(t *testing.T) {
	assertPanics(t, func() {
		pbt.AnalyzeDistribution[int](nil, func(_ int) string { return "x" }, 10, 1, 10)
	})
	assertPanics(t, func() {
		pbt.AnalyzeDistribution(pbt.Int(), nil, 10, 1, 10)
	})
	assertPanics(t, func() {
		pbt.AnalyzeDistribution(pbt.Int(), func(_ int) string { return "x" }, 0, 1, 10)
	})

	// maxSize <= 0 falls back to a default instead of panicking.
	report := pbt.AnalyzeDistribution(
		pbt.IntRange(0, 3),
		func(v int) string {
			if v < 2 {
				return "low"
			}
			return "high"
		},
		50, 1, 0,
	)
	if report.Total != 50 {
		t.Fatalf("unexpected total: %d", report.Total)
	}
	sum := 0
	for _, c := range report.Buckets {
		sum += c
	}
	if sum != 50 {
		t.Fatalf("bucket counts must sum to runs: %d", sum)
	}
}

func TestValidateDistributionErrors(t *testing.T) {
	report := pbt.DistributionReport{Total: 10, Buckets: map[string]int{"a": 5}}

	if err := pbt.ValidateDistribution(pbt.DistributionReport{}, nil); err == nil {
		t.Fatalf("expected error for empty report")
	}
	if err := pbt.ValidateDistribution(report, []pbt.DistributionRule{{Bucket: ""}}); err == nil {
		t.Fatalf("expected error for empty bucket")
	}
	if err := pbt.ValidateDistribution(report, []pbt.DistributionRule{{Bucket: "a", MinPercent: 80, MaxPercent: 20}}); err == nil {
		t.Fatalf("expected error for min > max")
	}
	if err := pbt.ValidateDistribution(report, []pbt.DistributionRule{{Bucket: "a", MinPercent: 60, MaxPercent: 100}}); err == nil {
		t.Fatalf("expected error for bucket above max bound")
	}
	if err := pbt.ValidateDistribution(report, []pbt.DistributionRule{{Bucket: "a", MinPercent: 40, MaxPercent: 60}}); err != nil {
		t.Fatalf("unexpected error for in-range bucket: %v", err)
	}
}

func TestSortedBucketsDeterministic(t *testing.T) {
	report := pbt.DistributionReport{Total: 3, Buckets: map[string]int{"b": 1, "a": 1, "c": 1}}
	got := report.SortedBuckets()
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("buckets not sorted: %v", got)
	}
}

func TestStatefulValidationPanics(t *testing.T) {
	valid := func() pbt.CommandModel[int] {
		return pbt.CommandModel[int]{
			Name:      "m",
			Init:      func(_ *rand.Rand) int { return 0 },
			Commands:  []pbt.StatefulCommand[int]{addCommand{}},
			Invariant: func(_ int) bool { return true },
		}
	}

	assertPanics(t, func() {
		m := valid()
		m.Init = nil
		pbt.CheckStatefulResult(m)
	})
	assertPanics(t, func() {
		m := valid()
		m.Invariant = nil
		pbt.CheckStatefulResult(m)
	})
	assertPanics(t, func() {
		m := valid()
		m.Commands = nil
		pbt.CheckStatefulResult(m)
	})
	assertPanics(t, func() {
		m := valid()
		m.Commands = []pbt.StatefulCommand[int]{addCommand{}, nil}
		pbt.CheckStatefulResult(m)
	})
	assertPanics(t, func() {
		pbt.CheckStatefulResult(valid(), pbt.Option(func(c *pbt.Config) { c.Timeout = -time.Second }))
	})
	assertPanics(t, func() {
		pbt.CheckStatefulResult(valid(), pbt.Option(func(c *pbt.Config) { c.ShrinkParallelism = 0 }))
	})
	assertPanics(t, func() {
		pbt.CheckStatefulResult(valid(), pbt.WithRuns(0))
	})
}

type sleepyCommand struct {
	d time.Duration
}

func (s sleepyCommand) Name() string            { return "sleepy" }
func (s sleepyCommand) Precondition(_ int) bool { return true }
func (s sleepyCommand) Next(_ *rand.Rand, state int) int {
	time.Sleep(s.d)
	return state + 1
}

func TestStatefulTimeoutAbortsRun(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name:      "slow model",
		Init:      func(_ *rand.Rand) int { return 0 },
		Commands:  []pbt.StatefulCommand[int]{sleepyCommand{d: 15 * time.Millisecond}},
		Invariant: func(_ int) bool { return true },
	}

	result := pbt.CheckStatefulResult(
		model,
		pbt.WithRuns(8),
		pbt.WithMaxSize(8),
		pbt.WithSeed(1),
		pbt.WithTimeout(25*time.Millisecond),
	)
	if result.Passed {
		t.Fatalf("expected timeout to fail the run")
	}
	if !result.TimedOut {
		t.Fatalf("expected TimedOut flag")
	}
	if !strings.Contains(result.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %q", result.Error())
	}
}

func TestStatefulTimeoutDuringShrinkKeepsCounterexample(t *testing.T) {
	// The scenario fails after ~40ms of step sleeps. Shrinking then evaluates
	// many candidate replays, so the run crosses the timeout mid-shrink and
	// must keep the already recorded failing trace.
	model := pbt.CommandModel[int]{
		Name:      "slow shrink model",
		Init:      func(_ *rand.Rand) int { return 0 },
		Commands:  []pbt.StatefulCommand[int]{sleepyCommand{d: 5 * time.Millisecond}},
		Invariant: func(state int) bool { return state < 8 },
	}

	result := pbt.CheckStatefulResult(
		model,
		pbt.WithRuns(1),
		pbt.WithMaxSize(8),
		pbt.WithSeed(1),
		pbt.WithTimeout(150*time.Millisecond),
	)
	if result.Passed {
		t.Fatalf("expected failure")
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout during shrinking to be flagged")
	}
	if result.FailedCommand != "sleepy" {
		t.Fatalf("unexpected failed command: %q", result.FailedCommand)
	}
	if len(result.Trace) != 8 {
		t.Fatalf("expected unshrunk trace of 8 steps, got %v", result.Trace)
	}
	if !strings.Contains(result.Error(), "timeout during shrinking") {
		t.Fatalf("unexpected error message: %q", result.Error())
	}
}

func TestStatefulNoEnabledCommandsPasses(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name:      "dead model",
		Init:      func(_ *rand.Rand) int { return 0 },
		Commands:  []pbt.StatefulCommand[int]{neverEnabledCommand{}},
		Invariant: func(_ int) bool { return true },
	}
	result := pbt.CheckStatefulResult(model, pbt.WithRuns(10), pbt.WithSeed(3))
	if !result.Passed {
		t.Fatalf("expected pass when no command is enabled")
	}
}

type neverEnabledCommand struct{}

func (neverEnabledCommand) Name() string                     { return "never" }
func (neverEnabledCommand) Precondition(_ int) bool          { return false }
func (neverEnabledCommand) Next(_ *rand.Rand, state int) int { return state }
