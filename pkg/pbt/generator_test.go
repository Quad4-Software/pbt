// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

const testASCIIAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func TestIntRangeAlwaysWithinBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gen := pbt.IntRange(-12, 12)

	for range 1000 {
		v := gen.Generate(rng, 100)
		if v < -12 || v > 12 {
			t.Fatalf("generated value out of range: %d", v)
		}
	}
}

func TestStringASCIIGeneratesExpectedCharactersAndLength(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	gen := pbt.StringASCII(3, 12)

	for range 500 {
		value := gen.Generate(rng, 12)
		if len(value) < 3 || len(value) > 12 {
			t.Fatalf("generated string has invalid length: %d", len(value))
		}
		for _, r := range value {
			if !strings.ContainsRune(testASCIIAlphabet, r) {
				t.Fatalf("generated non-ascii-alnum rune: %q", r)
			}
		}
	}
}

func TestSliceOfRespectsBoundsAndElementGenerator(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	elem := pbt.IntRange(5, 7)
	gen := pbt.SliceOf(elem, 2, 8)

	for range 400 {
		out := gen.Generate(rng, 8)
		if len(out) < 2 || len(out) > 8 {
			t.Fatalf("invalid generated length: %d", len(out))
		}
		for _, v := range out {
			if v < 5 || v > 7 {
				t.Fatalf("invalid generated element: %d", v)
			}
		}
	}
}

func TestMapTransformsSourceValues(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	source := pbt.IntRange(0, 10)
	mapped := pbt.Map("double", source, func(v int) int {
		return v * 2
	})

	for range 200 {
		v := mapped.Generate(rng, 10)
		if v < 0 || v > 20 || v%2 != 0 {
			t.Fatalf("unexpected mapped value: %d", v)
		}
	}
}

func TestIntRangeExtremeBoundsProducesVariedValues(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	gen := pbt.IntRange(math.MinInt, math.MaxInt)

	first := gen.Generate(rng, 100)
	seenDifferent := false
	for range 64 {
		v := gen.Generate(rng, 100)
		if v != first {
			seenDifferent = true
			break
		}
	}

	if !seenDifferent {
		t.Fatalf("expected varied values for full int range")
	}
}

func TestIntRangeExtremeLowerWindowStaysInBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	lo := math.MinInt
	hi := math.MinInt + 8
	gen := pbt.IntRange(lo, hi)

	for range 300 {
		v := gen.Generate(rng, 100)
		if v < lo || v > hi {
			t.Fatalf("out of bounds value: %d", v)
		}
	}
}

func TestIntRangeExtremeUpperWindowStaysInBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	lo := math.MaxInt - 8
	hi := math.MaxInt
	gen := pbt.IntRange(lo, hi)

	for range 300 {
		v := gen.Generate(rng, 100)
		if v < lo || v > hi {
			t.Fatalf("out of bounds value: %d", v)
		}
	}
}

func TestSuchThatFiltersValues(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gen := pbt.SuchThat(
		"even ints",
		pbt.IntRange(0, 100),
		func(v int) bool { return v%2 == 0 },
		500,
	)
	for range 100 {
		v := gen.Generate(rng, 50)
		if v%2 != 0 {
			t.Fatalf("SuchThat produced odd value: %d", v)
		}
	}
}

func TestSuchThatPanicsWhenImpossible(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when predicate is impossible")
		}
	}()
	rng := rand.New(rand.NewSource(1))
	gen := pbt.SuchThat(
		"impossible",
		pbt.IntRange(1, 10),
		func(v int) bool { return v == 999 },
		10,
	)
	gen.Generate(rng, 5)
}

func TestSuchThatFallbackReturnsFallbackWhenExhausted(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fallback := 42
	gen := pbt.SuchThatFallback(
		"impossible with fallback",
		pbt.IntRange(1, 10),
		func(v int) bool { return v == 999 },
		fallback,
		10,
	)
	v := gen.Generate(rng, 5)
	if v != fallback {
		t.Fatalf("expected fallback %d when exhausted, got %d", fallback, v)
	}
}

func TestSuchThatFallbackReturnsGeneratedWhenFound(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gen := pbt.SuchThatFallback(
		"even with fallback",
		pbt.IntRange(0, 100),
		func(v int) bool { return v%2 == 0 },
		0,
		500,
	)
	for range 50 {
		v := gen.Generate(rng, 50)
		if v%2 != 0 {
			t.Fatalf("expected even value, got %d", v)
		}
	}
}
