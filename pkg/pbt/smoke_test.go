// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

// TestFailureReportEndToEnd is the realistic smoke test: a failing property
// must produce a complete report containing the seed, the failing run index,
// the shrunk counterexample, classifier labels, and a consistent shrink
// trace where every element still fails the predicate.
func TestFailureReportEndToEnd(t *testing.T) {
	predicate := func(v int) bool { return v == 0 }

	var originalValue int
	var originalIndex = -1
	var shrinkEvents []pbt.ShrinkStepEvent[int]
	hook := pbt.HookFuncs[int]{
		CaseGenerated: func(e pbt.CaseGeneratedEvent[int]) {
			if !e.Passed && originalIndex < 0 {
				originalIndex = e.Index
				originalValue = e.Value
			}
		},
		ShrinkStep: func(e pbt.ShrinkStepEvent[int]) {
			shrinkEvents = append(shrinkEvents, e)
		},
	}

	property := pbt.ForAll(
		"smoke report",
		pbt.IntRange(-1000, 1000),
		predicate,
		pbt.WithShrinker[int](pbt.IntShrinker()),
		pbt.WithClassifier(func(v int) []string {
			return []string{fmt.Sprintf("sign=%d", sign(v))}
		}),
		pbt.WithHook[int](hook),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(64), pbt.WithSeed(123))

	if result.Passed || !result.HasCounterexample {
		t.Fatalf("expected a failing result with counterexample")
	}
	if originalIndex < 0 {
		t.Fatalf("hook never observed the failing case")
	}
	if result.FailureIndex != originalIndex {
		t.Fatalf("failure index mismatch: result=%d observed=%d", result.FailureIndex, originalIndex)
	}
	if predicate(result.Counterexample) {
		t.Fatalf("reported counterexample %d does not fail the predicate", result.Counterexample)
	}
	if len(result.FailureLabels) != 1 || result.FailureLabels[0] != fmt.Sprintf("sign=%d", sign(result.Counterexample)) {
		t.Fatalf("failure labels must describe the reported counterexample: %v", result.FailureLabels)
	}

	// The shrink trace must start at the originally generated value, end at
	// the reported counterexample, and contain only failing halving steps.
	trace := result.ShrinkTrace
	if len(trace) < 2 {
		t.Fatalf("expected a shrink trace, got %v", trace)
	}
	if trace[0] != originalValue {
		t.Fatalf("trace must start at the original value %d, got %d", originalValue, trace[0])
	}
	if trace[len(trace)-1] != result.Counterexample {
		t.Fatalf("trace must end at the counterexample %d, got %d", result.Counterexample, trace[len(trace)-1])
	}
	for i, v := range trace {
		if predicate(v) {
			t.Fatalf("trace element %d passes the predicate: %d", i, v)
		}
		if i > 0 && v != trace[i-1]/2 {
			t.Fatalf("trace step %d is not a halving: %d -> %d", i, trace[i-1], v)
		}
	}

	// Shrink step hook events must mirror the trace transitions.
	if len(shrinkEvents) != len(trace)-1 {
		t.Fatalf("expected %d shrink events, got %d", len(trace)-1, len(shrinkEvents))
	}
	for i, e := range shrinkEvents {
		if e.Step != i+1 || e.From != trace[i] || e.To != trace[i+1] {
			t.Fatalf("shrink event %d mismatch: %+v", i, e)
		}
	}

	// The rendered report must contain every key reproduction field.
	msg := result.Error()
	for _, fragment := range []string{
		`"smoke report"`,
		"seed=123",
		fmt.Sprintf("counterexample=%d", result.Counterexample),
		fmt.Sprintf("run %d of 64", result.FailureIndex),
		"sign=",
	} {
		if !strings.Contains(msg, fragment) {
			t.Fatalf("error report missing %q: %s", fragment, msg)
		}
	}
}

// TestFailureReportEndToEndParallel verifies the same report contract under
// partitioned execution.
func TestFailureReportEndToEndParallel(t *testing.T) {
	predicate := func(v int) bool { return v == 0 }

	property := pbt.ForAll(
		"smoke report parallel",
		pbt.IntRange(-500, 500),
		predicate,
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	result := pbt.CheckResult(
		property,
		pbt.WithRuns(200),
		pbt.WithSeed(55),
		pbt.WithParallelism(4),
	)

	if result.Passed || !result.HasCounterexample {
		t.Fatalf("expected a failing result with counterexample")
	}
	if result.FailureIndex < 0 || result.FailureIndex >= 200 {
		t.Fatalf("failure index out of range: %d", result.FailureIndex)
	}
	if predicate(result.Counterexample) {
		t.Fatalf("reported counterexample %d does not fail", result.Counterexample)
	}
	trace := result.ShrinkTrace
	if len(trace) < 2 || trace[len(trace)-1] != result.Counterexample {
		t.Fatalf("inconsistent shrink trace: %v -> %d", trace, result.Counterexample)
	}
	for i, v := range trace {
		if predicate(v) {
			t.Fatalf("trace element %d passes: %d", i, v)
		}
	}
	if !strings.Contains(result.Error(), "seed=55") {
		t.Fatalf("error report missing seed: %s", result.Error())
	}
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
