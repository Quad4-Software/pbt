// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt

import (
	"context"
	"math"
	"math/rand"
	"testing"
)

func TestSizeForRunBoundaries(t *testing.T) {
	if got := sizeForRun(0, 10, 5); got != 1 {
		t.Fatalf("first run must use size 1, got %d", got)
	}
	if got := sizeForRun(9, 10, 5); got != 5 {
		t.Fatalf("last run must reach maxSize, got %d", got)
	}
	if got := sizeForRun(0, 1, 7); got != 7 {
		t.Fatalf("single run must use maxSize, got %d", got)
	}
	if got := sizeForRun(4, 10, 1); got != 1 {
		t.Fatalf("maxSize 1 must clamp every run to 1, got %d", got)
	}
}

func TestSizeForRunMonotonicAndBounded(t *testing.T) {
	for runs := 1; runs <= 20; runs++ {
		for maxSize := 1; maxSize <= 20; maxSize++ {
			prev := 1
			for i := 0; i < runs; i++ {
				size := sizeForRun(i, runs, maxSize)
				if size < 1 || size > maxSize {
					t.Fatalf("size out of bounds: runs=%d maxSize=%d i=%d size=%d", runs, maxSize, i, size)
				}
				if size < prev {
					t.Fatalf("size not monotonic: runs=%d maxSize=%d i=%d size=%d prev=%d", runs, maxSize, i, size, prev)
				}
				prev = size
			}
			if runs > 1 && prev != maxSize {
				t.Fatalf("final size must equal maxSize: runs=%d maxSize=%d got=%d", runs, maxSize, prev)
			}
		}
	}
}

func TestPartitionSeedDistinctPerWorker(t *testing.T) {
	seeds := []int64{0, 1, -1, 42, math.MinInt64, math.MaxInt64}
	for _, seed := range seeds {
		seen := make(map[int64]int, 256)
		for w := 0; w < 256; w++ {
			s := partitionSeed(seed, w)
			if prev, dup := seen[s]; dup {
				t.Fatalf("partition seed collision: seed=%d workers=%d,%d", seed, prev, w)
			}
			seen[s] = w
		}
	}
}

func TestRemoveSeedRange(t *testing.T) {
	in := []int64{1, 2, 3, 4, 5}

	out := removeSeedRange(in, 1, 3)
	want := []int64{1, 4, 5}
	if len(out) != len(want) {
		t.Fatalf("unexpected removal result: %v", out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("unexpected removal result: %v", out)
		}
	}
	// The input slice must not be mutated.
	for i, v := range []int64{1, 2, 3, 4, 5} {
		if in[i] != v {
			t.Fatalf("input mutated: %v", in)
		}
	}

	if out := removeSeedRange(in, 0, len(in)); len(out) != 0 {
		t.Fatalf("removing everything must yield empty slice, got %v", out)
	}
	if out := removeSeedRange(in, 0, 0); len(out) != len(in) {
		t.Fatalf("empty removal must keep input, got %v", out)
	}
	if out := removeSeedRange(in, 4, 5); len(out) != 4 || out[3] != 4 {
		t.Fatalf("suffix removal wrong: %v", out)
	}
}

func TestShrinkWithTraceRejectsPassingResult(t *testing.T) {
	// A misbehaving shrinker that returns a passing value must be discarded so
	// the reported counterexample still reproduces the failure.
	bad := ShrinkerFunc[int](func(_ int, _ Predicate[int]) (int, bool) {
		return 0, true
	})
	predicate := func(v int) bool { return v == 0 }

	final, trace := shrinkWithTrace(bad, 7, predicate, 1)
	if final != 7 {
		t.Fatalf("expected original counterexample to be kept, got %d", final)
	}
	if trace != nil {
		t.Fatalf("expected nil trace for discarded shrink, got %v", trace)
	}
}

func TestShrinkWithTraceSynthesizesTraceForPlainShrinker(t *testing.T) {
	plain := ShrinkerFunc[int](func(v int, _ Predicate[int]) (int, bool) {
		return v - 1, true
	})
	predicate := func(v int) bool { return v < 5 }

	final, trace := shrinkWithTrace(plain, 10, predicate, 1)
	if final != 9 {
		t.Fatalf("expected shrunk value 9, got %d", final)
	}
	if len(trace) != 2 || trace[0] != 10 || trace[1] != 9 {
		t.Fatalf("expected synthesized two-step trace, got %v", trace)
	}
}

func TestShrinkWithTraceUnchangedShrinker(t *testing.T) {
	idle := ShrinkerFunc[int](func(v int, _ Predicate[int]) (int, bool) {
		return v, false
	})
	predicate := func(v int) bool { return v < 5 }

	final, trace := shrinkWithTrace(idle, 10, predicate, 1)
	if final != 10 {
		t.Fatalf("expected original value, got %d", final)
	}
	if trace != nil {
		t.Fatalf("expected nil trace, got %v", trace)
	}
}

type recordingParallelShrinker struct {
	seenWorkers int
}

func (r *recordingParallelShrinker) Shrink(v int, _ Predicate[int]) (int, bool) {
	return v, false
}

func (r *recordingParallelShrinker) ShrinkTraceParallel(v int, _ Predicate[int], workers int) ([]int, int, bool) {
	r.seenWorkers = workers
	return nil, v, false
}

func TestShrinkWithTraceClampsWorkerCount(t *testing.T) {
	rec := &recordingParallelShrinker{}
	predicate := func(v int) bool { return v < 5 }
	if _, _ = shrinkWithTrace(rec, 10, predicate, 0); rec.seenWorkers != 1 {
		t.Fatalf("expected worker count clamped to 1, got %d", rec.seenWorkers)
	}
}

func TestTimeoutAwarePredicateReportsPassingWhenDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	wrapped := timeoutAwarePredicate(ctx, func(_ int) bool {
		calls++
		return false
	})

	if wrapped(1) {
		t.Fatalf("expected wrapped predicate to forward false")
	}
	if calls != 1 {
		t.Fatalf("expected underlying predicate to be called once, got %d", calls)
	}

	cancel()
	if !wrapped(1) {
		t.Fatalf("expected wrapped predicate to report passing after cancel")
	}
	if calls != 1 {
		t.Fatalf("underlying predicate must not be called after cancel, got %d", calls)
	}
}

func TestEvaluateCoverageThresholdSemantics(t *testing.T) {
	labels := map[string]int{"a": 3, "b": 0}
	buckets := map[string]int{"x": 10}

	errs := evaluateCoverage(CoverageConfig{
		LabelRules: []CoverageRule{
			{Key: "a", MinCount: 4},              // 3 < 4 fails
			{Key: "b", MinPercent: 1},            // 0/20 fails
			{Key: "a", MinPercent: 10},           // 15% ok
			{Key: "missing", MinCount: 1},        // absent key fails
		},
		BucketRules: []CoverageRule{
			{Key: "x", MinPercent: 50}, // exactly 50% passes
		},
	}, 20, labels, buckets)

	if len(errs) != 3 {
		t.Fatalf("expected 3 coverage errors, got %v", errs)
	}
	for i := 1; i < len(errs); i++ {
		if errs[i] < errs[i-1] {
			t.Fatalf("coverage errors must be sorted: %v", errs)
		}
	}

	if errs := evaluateCoverage(CoverageConfig{
		LabelRules: []CoverageRule{{Key: "a", MinCount: 100}},
	}, 0, labels, buckets); errs != nil {
		t.Fatalf("zero runs must skip coverage evaluation, got %v", errs)
	}
}

func TestMeetsCoverageRuleBoundaries(t *testing.T) {
	if !meetsCoverageRule(CoverageRule{Key: "k", MinCount: 5}, 5, 10) {
		t.Fatalf("exact MinCount must pass")
	}
	if meetsCoverageRule(CoverageRule{Key: "k", MinCount: 6}, 5, 10) {
		t.Fatalf("below MinCount must fail")
	}
	if !meetsCoverageRule(CoverageRule{Key: "k", MinPercent: 50}, 5, 10) {
		t.Fatalf("exact MinPercent must pass")
	}
	if meetsCoverageRule(CoverageRule{Key: "k", MinPercent: 50.1}, 5, 10) {
		t.Fatalf("below MinPercent must fail")
	}
}

func TestParallelFindFirstFailingRepansicsDeterministically(t *testing.T) {
	model := CommandModel[int]{
		Name: "panic model",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []StatefulCommand[int]{
			alwaysInc{},
		},
		// Every two-step replay reaches state 2 and panics inside workers.
		Invariant: func(state int) bool {
			if state >= 2 {
				panic("invariant exploded")
			}
			return true
		},
	}

	current := []int64{11, 22, 33, 44}
	defer func() {
		r := recover()
		if r != "invariant exploded" {
			t.Fatalf("expected re-raised panic, got %v", r)
		}
	}()
	parallelFindFirstFailing(context.Background(), model, 7, current, 1, []int{0, 1, 2, 3}, 3)
}

type alwaysInc struct{}

func (alwaysInc) Name() string                     { return "inc" }
func (alwaysInc) Precondition(_ int) bool          { return true }
func (alwaysInc) Next(_ *rand.Rand, state int) int { return state + 1 }
