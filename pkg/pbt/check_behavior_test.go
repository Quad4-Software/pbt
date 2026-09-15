// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestCheckUsesFatalOnFailure(t *testing.T) {
	property := pbt.ForAll(
		"always fails",
		pbt.IntRange(1, 1),
		func(_ int) bool { return false },
	)

	testDouble := &fatalRecorder{}
	assertPanics(t, func() {
		pbt.Check(testDouble, property, pbt.WithRuns(1), pbt.WithSeed(1))
	})

	if !testDouble.called {
		t.Fatalf("expected Fatalf to be called")
	}
	if !strings.Contains(testDouble.msg, "always fails") {
		t.Fatalf("unexpected fatal message: %s", testDouble.msg)
	}
}

func TestCheckResultDeterministicForSameSeed(t *testing.T) {
	property := pbt.ForAll(
		"same seed gives same result",
		pbt.IntRange(-50, 50),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	left := pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(99))
	right := pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(99))

	if left.Passed != right.Passed {
		t.Fatalf("pass mismatch: %v vs %v", left.Passed, right.Passed)
	}
	if left.Counterexample != right.Counterexample {
		t.Fatalf("counterexample mismatch: %v vs %v", left.Counterexample, right.Counterexample)
	}
	if left.Error() != right.Error() {
		t.Fatalf("error mismatch: %q vs %q", left.Error(), right.Error())
	}
}

func TestCheckResultTimesOut(t *testing.T) {
	property := pbt.ForAll(
		"slow predicate timeout",
		pbt.IntRange(0, 10),
		func(_ int) bool {
			time.Sleep(2 * time.Millisecond)
			return true
		},
	)

	result := pbt.CheckResult(
		property,
		pbt.WithRuns(1000),
		pbt.WithSeed(10),
		pbt.WithTimeout(time.Millisecond),
	)

	if result.Passed {
		t.Fatalf("expected timeout result to fail")
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout result")
	}
	if !strings.Contains(result.Error(), "timed out") {
		t.Fatalf("expected timeout error message, got %q", result.Error())
	}
}

func TestCheckResultPanicsOnInvalidPropertyName(t *testing.T) {
	property := pbt.ForAll(
		"",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
	)

	assertPanics(t, func() {
		_ = pbt.CheckResult(property, pbt.WithSeed(1))
	})
}

func TestCheckResultPanicsOnNilGenerator(t *testing.T) {
	property := pbt.Property[int]{
		Name:      "nil generator",
		Generator: nil,
		Predicate: func(_ int) bool { return true },
	}

	assertPanics(t, func() {
		_ = pbt.CheckResult(property, pbt.WithSeed(1))
	})
}

func TestCheckResultPanicsOnNilPredicate(t *testing.T) {
	property := pbt.Property[int]{
		Name:      "nil predicate",
		Generator: pbt.IntRange(0, 1),
		Predicate: nil,
	}

	assertPanics(t, func() {
		_ = pbt.CheckResult(property, pbt.WithSeed(1))
	})
}

func TestResultErrorFormatsFailure(t *testing.T) {
	property := pbt.ForAll(
		"failure format",
		pbt.IntRange(4, 4),
		func(v int) bool { return v == 0 },
	)

	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(3))
	if result.Passed {
		t.Fatalf("expected failing result")
	}
	if !strings.Contains(result.Error(), "failure format") {
		t.Fatalf("missing property name in error: %q", result.Error())
	}
}

type fatalRecorder struct {
	called bool
	msg    string
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(format string, args ...any) {
	f.called = true
	f.msg = fmt.Sprintf(format, args...)
	panic("fatal called")
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()

	fn()
}
