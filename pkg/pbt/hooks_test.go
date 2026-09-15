// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestHookLifecycleEventsForFailureAndShrink(t *testing.T) {
	rec := &intHookRecorder{}

	property := pbt.ForAll(
		"hook lifecycle",
		pbt.IntRange(64, 64),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
		pbt.WithHook[int](rec),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(1))
	if result.Passed {
		t.Fatalf("expected failure")
	}

	if rec.runStartCount != 1 {
		t.Fatalf("expected one run start event")
	}
	if rec.runEndCount != 1 {
		t.Fatalf("expected one run end event")
	}
	if rec.caseCount != 1 {
		t.Fatalf("expected one case event")
	}
	if rec.failureCount != 1 {
		t.Fatalf("expected one failure event")
	}
	if rec.shrinkStepCount == 0 {
		t.Fatalf("expected shrink step events")
	}
}

func TestHookReplayEvent(t *testing.T) {
	rec := &intHookRecorder{}
	property := pbt.ForAll(
		"hook replay",
		pbt.IntRange(7, 7),
		func(v int) bool { return v == 0 },
		pbt.WithHook[int](rec),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(2))
	if result.Passed {
		t.Fatalf("expected failing result")
	}
	fixture, err := result.ToReplayFixture()
	if err != nil {
		t.Fatalf("unexpected fixture conversion error: %v", err)
	}

	replayed, err := pbt.ReplayFixtureValue(property, fixture)
	if err != nil {
		t.Fatalf("unexpected replay error: %v", err)
	}
	if !replayed.Passed {
		t.Fatalf("expected replay operation to pass")
	}
	if rec.replayCount != 1 {
		t.Fatalf("expected one replay event")
	}
}

func TestHookCaseEventsUnderParallelExecution(t *testing.T) {
	rec := &intHookRecorder{}
	runs := 200

	property := pbt.ForAll(
		"parallel case events",
		pbt.IntRange(1, 1),
		func(_ int) bool { return true },
		pbt.WithHook[int](rec),
	)

	result := pbt.CheckResult(
		property,
		pbt.WithRuns(runs),
		pbt.WithSeed(9),
		pbt.WithParallelism(8),
	)
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if rec.caseCount != runs {
		t.Fatalf("expected %d case events, got %d", runs, rec.caseCount)
	}
}

func TestHookDispatchOrder(t *testing.T) {
	var order []string
	hook := pbt.HookFuncs[int]{
		RunStart:      func(_ pbt.RunStartEvent) { order = append(order, "start") },
		CaseGenerated: func(_ pbt.CaseGeneratedEvent[int]) { order = append(order, "case") },
		Failure:       func(_ pbt.FailureEvent[int]) { order = append(order, "failure") },
		ShrinkStep:    func(_ pbt.ShrinkStepEvent[int]) { order = append(order, "shrink") },
		RunEnd:        func(_ pbt.RunEndEvent) { order = append(order, "end") },
	}

	property := pbt.ForAll(
		"hook order",
		pbt.IntRange(8, 8),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
		pbt.WithHook[int](hook),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(1))
	if result.Passed {
		t.Fatalf("expected failure")
	}

	// Expected sequence: start, case, failure, one or more shrink, end.
	if len(order) < 5 {
		t.Fatalf("incomplete lifecycle: %v", order)
	}
	if order[0] != "start" {
		t.Fatalf("run start must be first: %v", order)
	}
	if order[len(order)-1] != "end" {
		t.Fatalf("run end must be last: %v", order)
	}
	if order[1] != "case" || order[2] != "failure" {
		t.Fatalf("case and failure must precede shrinking: %v", order)
	}
	for _, e := range order[3 : len(order)-1] {
		if e != "shrink" {
			t.Fatalf("unexpected event between failure and end: %v", order)
		}
	}
}

func TestMultipleHooksAllReceiveEvents(t *testing.T) {
	first := &intHookRecorder{}
	second := &intHookRecorder{}

	property := pbt.ForAll(
		"two hooks",
		pbt.IntRange(1, 1),
		func(_ int) bool { return true },
		pbt.WithHooks[int](first, second),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(10), pbt.WithSeed(1))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	for _, rec := range []*intHookRecorder{first, second} {
		if rec.runStartCount != 1 || rec.runEndCount != 1 || rec.caseCount != 10 {
			t.Fatalf("hook missed events: %+v", rec)
		}
	}
}

type intHookRecorder struct {
	mu sync.Mutex

	runStartCount   int
	caseCount       int
	failureCount    int
	shrinkStepCount int
	runEndCount     int
	replayCount     int
}

func (r *intHookRecorder) OnRunStart(_ pbt.RunStartEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runStartCount++
}

func (r *intHookRecorder) OnCaseGenerated(_ pbt.CaseGeneratedEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.caseCount++
}

func (r *intHookRecorder) OnFailure(_ pbt.FailureEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failureCount++
}

func (r *intHookRecorder) OnShrinkStep(_ pbt.ShrinkStepEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shrinkStepCount++
}

func (r *intHookRecorder) OnRunEnd(_ pbt.RunEndEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runEndCount++
}

func (r *intHookRecorder) OnReplay(_ pbt.ReplayEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replayCount++
}

type statefulAddCmd struct{}

func (statefulAddCmd) Name() string                     { return "add" }
func (statefulAddCmd) Precondition(_ int) bool          { return true }
func (statefulAddCmd) Next(_ *rand.Rand, state int) int { return state + 1 }

type statefulNoopCmd struct{}

func (statefulNoopCmd) Name() string                     { return "noop" }
func (statefulNoopCmd) Precondition(_ int) bool          { return true }
func (statefulNoopCmd) Next(_ *rand.Rand, state int) int { return state }

func TestStatefulHookEvents(t *testing.T) {
	rec := &statefulHookRecorder{}
	model := pbt.CommandModel[int]{
		Name: "stateful hooks",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			statefulNoopCmd{},
			statefulAddCmd{},
		},
		Invariant: func(state int) bool {
			return state < 3
		},
		Hooks: []pbt.StatefulHook[int]{rec},
	}

	var result pbt.StatefulResult[int]
	foundShrinkable := false
	for seed := int64(1); seed <= 300; seed++ {
		result = pbt.CheckStatefulResult(
			model,
			pbt.WithSeed(seed),
			pbt.WithRuns(1),
			pbt.WithMaxSize(32),
			pbt.WithShrinkParallelism(2),
		)
		if !result.Passed && result.ShrinkPasses > 0 {
			foundShrinkable = true
			break
		}
	}
	if !foundShrinkable {
		t.Fatalf("expected to find a shrinkable stateful failure")
	}
	if rec.runStartCount < 1 {
		t.Fatalf("expected stateful run-start events")
	}
	if rec.stepCount == 0 {
		t.Fatalf("expected step events")
	}
	if rec.failureCount < 1 {
		t.Fatalf("expected failure events")
	}
	if rec.shrinkPassCount == 0 || result.ShrinkPasses == 0 {
		t.Fatalf("expected shrink-pass events")
	}
	if rec.runEndCount < 1 {
		t.Fatalf("expected run-end events")
	}

	fixture, err := result.ToStatefulReplayFixture()
	if err != nil {
		t.Fatalf("unexpected fixture conversion error: %v", err)
	}
	replayed := pbt.ReplayStatefulFixture(model, fixture)
	if replayed.Passed {
		t.Fatalf("expected replay to fail")
	}
	if rec.replayCount < 1 {
		t.Fatalf("expected replay events")
	}
}

type statefulHookRecorder struct {
	mu sync.Mutex

	runStartCount   int
	stepCount       int
	failureCount    int
	shrinkPassCount int
	runEndCount     int
	replayCount     int
}

func (r *statefulHookRecorder) OnStatefulRunStart(_ pbt.StatefulRunStartEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runStartCount++
}

func (r *statefulHookRecorder) OnStatefulStep(_ pbt.StatefulStepEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepCount++
}

func (r *statefulHookRecorder) OnStatefulFailure(_ pbt.StatefulFailureEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failureCount++
}

func (r *statefulHookRecorder) OnStatefulShrinkPass(_ pbt.StatefulShrinkPassEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shrinkPassCount++
}

func (r *statefulHookRecorder) OnStatefulRunEnd(_ pbt.StatefulRunEndEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runEndCount++
}

func (r *statefulHookRecorder) OnStatefulReplay(_ pbt.StatefulReplayEvent[int]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replayCount++
}
