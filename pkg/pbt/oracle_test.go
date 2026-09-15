// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

// assertIntShrinkLocalMinimum verifies the shrink contract independently of
// the implementation: the result must still fail the predicate, and the next
// halving step must pass, so the value sits on a greedy halving boundary.
func assertIntShrinkLocalMinimum(t *testing.T, shrunk int, predicate func(int) bool) {
	t.Helper()
	if predicate(shrunk) {
		t.Fatalf("shrunk value %d must still fail the predicate", shrunk)
	}
	if shrunk != 0 && !predicate(shrunk/2) {
		t.Fatalf("shrunk value %d is not on a halving boundary: %d still fails", shrunk, shrunk/2)
	}
}

func TestIntShrinkerMatchesLocalMinimumOracle(t *testing.T) {
	shrinker := pbt.IntShrinker()

	cases := []struct {
		name      string
		value     int
		predicate func(int) bool
	}{
		{"threshold", 500, func(v int) bool { return v <= 30 }},
		{"negative threshold", -999, func(v int) bool { return v >= -30 }},
		{"modulo", 770, func(v int) bool { return v%11 != 0 }},
		{"nonzero", -64, func(v int) bool { return v == 0 }},
		{"band", 150, func(v int) bool { return v < 100 || v > 200 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.predicate(tc.value) {
				t.Fatalf("test setup broken: input must fail predicate")
			}
			shrunk, _ := shrinker.Shrink(tc.value, tc.predicate)
			assertIntShrinkLocalMinimum(t, shrunk, tc.predicate)
			if (tc.value > 0 && shrunk < 0) || (tc.value < 0 && shrunk > 0) {
				t.Fatalf("halving must preserve sign: %d -> %d", tc.value, shrunk)
			}
			if abs(shrunk) > abs(tc.value) {
				t.Fatalf("shrink moved away from zero: %d -> %d", tc.value, shrunk)
			}
		})
	}
}

// assertSliceShrinkLocalMinimum verifies the shrunk slice is a local minimum:
// it still fails, and no single halving or one-element removal fails again.
func assertSliceShrinkLocalMinimum[T any](t *testing.T, final []T, predicate func([]T) bool) {
	t.Helper()
	if predicate(final) {
		t.Fatalf("shrunk slice must still fail the predicate")
	}
	if len(final) > 1 {
		half := len(final) / 2
		if !predicate(final[:half]) {
			t.Fatalf("first half %v still fails: %v is not a local minimum", final[:half], final)
		}
		if !predicate(final[half:]) {
			t.Fatalf("second half %v still fails: %v is not a local minimum", final[half:], final)
		}
	}
	for i := range final {
		shorter := make([]T, 0, len(final)-1)
		shorter = append(shorter, final[:i]...)
		shorter = append(shorter, final[i+1:]...)
		if !predicate(shorter) {
			t.Fatalf("removal at %d still fails: %v is not a local minimum", i, final)
		}
	}
}

func TestSliceShrinkerMatchesLocalMinimumOracle(t *testing.T) {
	shrinker := pbt.SliceShrinker[int]()

	cases := []struct {
		name      string
		value     []int
		predicate func([]int) bool
	}{
		{"length bound", []int{1, 2, 3, 4, 5, 6, 7, 8, 9}, func(s []int) bool { return len(s) <= 3 }},
		{"sum bound", []int{9, 9, 9, 9, 9, 9}, func(s []int) bool {
			sum := 0
			for _, v := range s {
				sum += v
			}
			return sum <= 20
		}},
		{"contains marker", []int{1, 7, 1, 1, 1, 1}, func(s []int) bool {
			for _, v := range s {
				if v == 7 {
					return false
				}
			}
			return true
		}},
		{"single element", []int{42}, func(s []int) bool { return len(s) == 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.predicate(tc.value) {
				t.Fatalf("test setup broken: input must fail predicate")
			}
			shrunk, _ := shrinker.Shrink(tc.value, tc.predicate)
			assertSliceShrinkLocalMinimum(t, shrunk, tc.predicate)
		})
	}
}

// naiveSubsequence reports whether candidate is a subsequence of value,
// checked independently of the shrinker implementation.
func naiveSubsequence[T comparable](candidate []T, value []T) bool {
	i := 0
	for _, v := range value {
		if i < len(candidate) && candidate[i] == v {
			i++
		}
	}
	return i == len(candidate)
}

func TestSliceShrinkerProducesSubsequence(t *testing.T) {
	shrinker := pbt.SliceShrinker[int]()
	predicate := func(s []int) bool { return len(s) <= 2 }

	value := []int{5, 1, 9, 2, 8, 3, 7}
	shrunk, changed := shrinker.Shrink(value, predicate)
	if !changed {
		t.Fatalf("expected shrinker to change the slice")
	}
	if !naiveSubsequence(shrunk, value) {
		t.Fatalf("shrunk slice %v is not a subsequence of %v", shrunk, value)
	}
}

func TestIntRangeBoundaryCoverageOracle(t *testing.T) {
	// Independent contract: a 4-value range must hit every bound given a
	// reasonable number of draws.
	rng := rand.New(rand.NewSource(1234))
	gen := pbt.IntRange(0, 3)
	seen := map[int]int{}
	for range 2000 {
		v := gen.Generate(rng, 10)
		if v < 0 || v > 3 {
			t.Fatalf("out of range value: %d", v)
		}
		seen[v]++
	}
	for i := 0; i <= 3; i++ {
		if seen[i] == 0 {
			t.Fatalf("range boundary value %d never generated", i)
		}
	}
}

func TestGeneratorDeterminismOracle(t *testing.T) {
	// Two identically seeded streams must produce identical values for every
	// built-in generator.
	gens := []pbt.Generator[int]{
		pbt.Int(),
		pbt.IntRange(-1000, 1000),
		pbt.IntRange(-5, -5),
	}
	for _, gen := range gens {
		left := rand.New(rand.NewSource(555))
		right := rand.New(rand.NewSource(555))
		for i := range 100 {
			a := gen.Generate(left, i)
			b := gen.Generate(right, i)
			if a != b {
				t.Fatalf("generator %s not deterministic at draw %d: %d vs %d", gen.Name(), i, a, b)
			}
		}
	}

	strLeft := rand.New(rand.NewSource(9))
	strRight := rand.New(rand.NewSource(9))
	strGen := pbt.StringASCII(0, 32)
	for i := range 100 {
		if a, b := strGen.Generate(strLeft, i), strGen.Generate(strRight, i); a != b {
			t.Fatalf("string generator not deterministic at draw %d", i)
		}
	}
}

func TestFrequencyWeightingOracle(t *testing.T) {
	// Independent contract: a 1:3 weighting must produce roughly 25%/75%.
	rng := rand.New(rand.NewSource(31415))
	gen := pbt.Frequency("weighted",
		pbt.WeightedGenerator[int]{Weight: 1, Generator: pbt.IntRange(0, 0)},
		pbt.WeightedGenerator[int]{Weight: 3, Generator: pbt.IntRange(1, 1)},
	)
	high := 0
	total := 4000
	for range total {
		if gen.Generate(rng, 10) == 1 {
			high++
		}
	}
	ratio := float64(high) / float64(total)
	if ratio < 0.65 || ratio > 0.85 {
		t.Fatalf("weighted selection ratio off: got %.3f expected ~0.75", ratio)
	}
}

func TestOneOfUniformityOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(2718))
	gen := pbt.OneOf("pick",
		pbt.IntRange(0, 0),
		pbt.IntRange(1, 1),
		pbt.IntRange(2, 2),
	)
	counts := map[int]int{}
	total := 3000
	for range total {
		counts[gen.Generate(rng, 10)]++
	}
	for i := 0; i < 3; i++ {
		ratio := float64(counts[i]) / float64(total)
		if ratio < 0.25 || ratio > 0.42 {
			t.Fatalf("OneOf selection skewed for value %d: %.3f", i, ratio)
		}
	}
}

func TestStringASCIILengthDistributionOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(1618))
	gen := pbt.StringASCII(0, 5)
	counts := map[int]int{}
	total := 6000
	for range total {
		v := gen.Generate(rng, 100)
		counts[len(v)]++
	}
	expected := float64(total) / 6.0
	for i := 0; i <= 5; i++ {
		if float64(counts[i]) < expected*0.5 || float64(counts[i]) > expected*1.5 {
			t.Fatalf("string length %d poorly distributed: %d of %d", i, counts[i], total)
		}
	}
}
