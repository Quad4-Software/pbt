// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

// goroutineSlack bounds background goroutine fluctuation tolerated by the
// leak checks below.
const goroutineSlack = 4

// waitForGoroutines polls the runtime goroutine count until it drops to want
// or the deadline passes, returning the last observed count.
func waitForGoroutines(want int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	last := runtime.NumGoroutine()
	for {
		runtime.GC()
		runtime.Gosched()
		last = runtime.NumGoroutine()
		if last <= want || time.Now().After(deadline) {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertNoGoroutineGrowth fails when more than slack goroutines remain
// reachable above baseline after a settle window.
func assertNoGoroutineGrowth(t *testing.T, baseline int) {
	t.Helper()
	after := waitForGoroutines(baseline+goroutineSlack, 3*time.Second)
	if after > baseline+goroutineSlack {
		t.Fatalf("possible goroutine leak detected: baseline=%d after=%d", baseline, after)
	}
}

func TestGoroutineDetectorObservesDeliberateLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	release := make(chan struct{})
	go func() { <-release }()
	defer close(release)

	// The detector must report the leaked goroutine while it is blocked.
	deadline := time.Now().Add(2 * time.Second)
	observed := baseline
	for time.Now().Before(deadline) {
		runtime.Gosched()
		observed = runtime.NumGoroutine()
		if observed > baseline {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if observed <= baseline {
		t.Fatalf("detector did not observe leaked goroutine: baseline=%d observed=%d", baseline, observed)
	}
}

func TestCheckResultTimeoutNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	property := pbt.ForAll(
		"timeout leak check",
		pbt.IntRange(0, 10),
		func(_ int) bool {
			time.Sleep(2 * time.Millisecond)
			return true
		},
	)

	for i := range 60 {
		result := pbt.CheckResult(
			property,
			pbt.WithRuns(200),
			pbt.WithSeed(int64(i+1)),
			pbt.WithTimeout(time.Millisecond),
		)
		if !result.TimedOut {
			t.Fatalf("expected timeout at iteration %d", i)
		}
	}

	assertNoGoroutineGrowth(t, baseline)
}

func TestCheckResultParallelTimeoutNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	property := pbt.ForAll(
		"parallel timeout leak check",
		pbt.IntRange(0, 10),
		func(_ int) bool {
			time.Sleep(2 * time.Millisecond)
			return true
		},
	)

	for i := range 20 {
		result := pbt.CheckResult(
			property,
			pbt.WithRuns(500),
			pbt.WithSeed(int64(i+1)),
			pbt.WithTimeout(time.Millisecond),
			pbt.WithParallelism(8),
		)
		if !result.TimedOut {
			t.Fatalf("expected timeout at iteration %d", i)
		}
	}

	assertNoGoroutineGrowth(t, baseline)
}

func TestStatefulShrinkParallelNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	// A model that fails on every run exercises the parallel shrink workers
	// and verifies they are always joined.
	model := pbt.CommandModel[int]{
		Name: "leak model",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			addCommand{},
		},
		Invariant: func(state int) bool { return state < 4 },
	}

	for seed := int64(1); seed <= 10; seed++ {
		result := pbt.CheckStatefulResult(
			model,
			pbt.WithSeed(seed),
			pbt.WithRuns(2),
			pbt.WithMaxSize(16),
			pbt.WithShrinkParallelism(4),
		)
		if result.Passed {
			t.Fatalf("expected stateful failure at seed %d", seed)
		}
	}

	assertNoGoroutineGrowth(t, baseline)
}
