// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestIntShrinkerKeepsFailureAndMovesTowardZero(t *testing.T) {
	shrinker := pbt.IntShrinker()
	predicate := func(v int) bool { return v >= 10 }

	shrunk, changed := shrinker.Shrink(-120, predicate)
	if !changed {
		t.Fatalf("expected shrinker to report change")
	}
	if shrunk >= 10 {
		t.Fatalf("shrunk value should still fail predicate, got %d", shrunk)
	}
	if abs(shrunk) >= abs(-120) {
		t.Fatalf("shrunk value should be closer to zero, got %d", shrunk)
	}
}

func TestIntShrinkerNoChangeOnPassingInput(t *testing.T) {
	shrinker := pbt.IntShrinker()
	predicate := func(v int) bool { return v >= 0 }

	_, changed := shrinker.Shrink(8, predicate)
	if changed {
		t.Fatalf("expected no change for passing input")
	}
}

func TestStringShrinkerKeepsFailureAndShortens(t *testing.T) {
	shrinker := pbt.StringShrinker()
	predicate := func(v string) bool { return v == "Z" }

	input := "    abcdefghijklmnop    "
	shrunk, changed := shrinker.Shrink(input, predicate)
	if !changed {
		t.Fatalf("expected change when shrinking failing string")
	}
	if len(shrunk) >= len(input) {
		t.Fatalf("expected shorter value, got %q", shrunk)
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk value should keep failing predicate, got %q", shrunk)
	}
}

func TestStringShrinkerNoChangeOnPassingInput(t *testing.T) {
	shrinker := pbt.StringShrinker()
	predicate := func(v string) bool { return len(v) > 0 }

	_, changed := shrinker.Shrink("abcdef", predicate)
	if changed {
		t.Fatalf("expected no change for passing input")
	}
}

func TestIntShrinkerMinimalityModuloPredicate(t *testing.T) {
	shrinker := pbt.IntShrinker()
	predicate := func(v int) bool { return v%7 == 0 }

	shrunk, changed := shrinker.Shrink(100, predicate)
	if !changed {
		t.Fatalf("expected shrinker to change value")
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk value must still fail predicate, got %d", shrunk)
	}
	if !predicate(shrunk / 2) {
		t.Fatalf("expected next halving step to pass-or-stop boundary, got shrunk=%d", shrunk)
	}
}

func TestIntShrinkerMinimalityThresholdPredicate(t *testing.T) {
	shrinker := pbt.IntShrinker()
	predicate := func(v int) bool {
		return v == 0
	}

	shrunk, changed := shrinker.Shrink(-1024, predicate)
	if !changed {
		t.Fatalf("expected shrinker to change value")
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk value must still fail predicate, got %d", shrunk)
	}
	if shrunk != -1 {
		t.Fatalf("expected minimal failing value near zero, got %d", shrunk)
	}
}

func TestStringShrinkerMinimalityLengthPredicate(t *testing.T) {
	shrinker := pbt.StringShrinker()
	predicate := func(v string) bool { return len(v) == 0 }

	shrunk, changed := shrinker.Shrink("abcdefghijklmnopqrstuvwxyz", predicate)
	if !changed {
		t.Fatalf("expected shrinker to change value")
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk value must still fail predicate")
	}
	if len(shrunk) != 1 {
		t.Fatalf("expected minimal non-empty failing value length, got %d", len(shrunk))
	}
}

func TestStringShrinkerMinimalityWhitespaceTrimmingPredicate(t *testing.T) {
	shrinker := pbt.StringShrinker()
	predicate := func(v string) bool {
		return strings.TrimSpace(v) == "x"
	}

	shrunk, changed := shrinker.Shrink("    abcdef    ", predicate)
	if !changed {
		t.Fatalf("expected shrinker to change value")
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk value must still fail predicate: %q", shrunk)
	}
	if len(shrunk) > 1 {
		t.Fatalf("expected very small failing output, got %q", shrunk)
	}
}

func TestSliceShrinkerShortensFailingSlice(t *testing.T) {
	shrinker := pbt.SliceShrinker[int]()
	predicate := func(s []int) bool { return len(s) <= 2 }

	shrunk, changed := shrinker.Shrink([]int{1, 2, 3, 4, 5}, predicate)
	if !changed {
		t.Fatalf("expected slice shrinker to change value")
	}
	if predicate(shrunk) {
		t.Fatalf("shrunk slice must still fail predicate")
	}
	if len(shrunk) >= 5 {
		t.Fatalf("expected shorter slice, got len=%d", len(shrunk))
	}
}

func TestSliceShrinkerNoChangeOnPassingInput(t *testing.T) {
	shrinker := pbt.SliceShrinker[int]()
	predicate := func(s []int) bool { return len(s) <= 10 }

	_, changed := shrinker.Shrink([]int{1, 2, 3}, predicate)
	if changed {
		t.Fatalf("expected no change for passing input")
	}
}
