// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestCheckResultPassesForValidProperty(t *testing.T) {
	property := pbt.ForAll(
		"absolute value is non-negative",
		pbt.IntRange(-100, 100),
		func(v int) bool {
			return abs(v) >= 0
		},
	)

	result := pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(7))
	if !result.Passed {
		t.Fatalf("expected pass, got failure: %s", result.Error())
	}
}

func TestCheckResultFindsCounterexample(t *testing.T) {
	property := pbt.ForAll(
		"all ints are zero",
		pbt.IntRange(-10, 10),
		func(v int) bool {
			return v == 0
		},
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(7))
	if result.Passed {
		t.Fatalf("expected failing property")
	}
	if !result.HasCounterexample {
		t.Fatalf("expected counterexample to be set")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
