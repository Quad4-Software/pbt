// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestMetaPropertyIntRangeStaysWithinBounds(t *testing.T) {
	property := pbt.ForAll(
		"int range generator stays bounded",
		pbt.IntRange(-500, 500),
		func(v int) bool {
			return v >= -500 && v <= 500
		},
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	pbt.Check(t, property, pbt.WithRuns(1200), pbt.WithSeed(11))
}

func TestMetaPropertyStringASCIIUsesExpectedAlphabet(t *testing.T) {
	alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	property := pbt.ForAll(
		"ascii generator emits allowed runes",
		pbt.StringASCII(0, 64),
		func(v string) bool {
			for _, r := range v {
				if !strings.ContainsRune(alphabet, r) {
					return false
				}
			}
			return len(v) <= 64
		},
		pbt.WithShrinker[string](pbt.StringShrinker()),
	)

	pbt.Check(t, property, pbt.WithRuns(1200), pbt.WithSeed(12))
}

func TestMetaPropertySliceOfRespectsLengthAndElementBounds(t *testing.T) {
	property := pbt.ForAll(
		"slice generator respects length and element ranges",
		pbt.SliceOf(pbt.IntRange(10, 20), 0, 25),
		func(values []int) bool {
			if len(values) > 25 {
				return false
			}
			for _, v := range values {
				if v < 10 || v > 20 {
					return false
				}
			}
			return true
		},
	)

	pbt.Check(t, property, pbt.WithRuns(1000), pbt.WithSeed(13))
}

func TestMetaPropertySameSeedProducesSameCheckResult(t *testing.T) {
	property := pbt.ForAll(
		"same seed deterministic outcome",
		pbt.IntRange(-200, 200),
		func(v int) bool {
			return v == 0
		},
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	left := pbt.CheckResult(property, pbt.WithRuns(600), pbt.WithSeed(2026))
	right := pbt.CheckResult(property, pbt.WithRuns(600), pbt.WithSeed(2026))

	if left.Passed != right.Passed {
		t.Fatalf("pass mismatch: %v != %v", left.Passed, right.Passed)
	}
	if left.Counterexample != right.Counterexample {
		t.Fatalf("counterexample mismatch: %d != %d", left.Counterexample, right.Counterexample)
	}
	if left.HasCounterexample != right.HasCounterexample {
		t.Fatalf("counterexample presence mismatch")
	}
}
