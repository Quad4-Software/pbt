// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestDefaultConfigHasExpectedValues(t *testing.T) {
	cfg := pbt.DefaultConfig()

	if cfg.Runs != 100 {
		t.Fatalf("unexpected runs: %d", cfg.Runs)
	}
	if cfg.MaxSize != 100 {
		t.Fatalf("unexpected max size: %d", cfg.MaxSize)
	}
	if cfg.Seed != 0 {
		t.Fatalf("expected default seed 0 for reproducibility, got %d", cfg.Seed)
	}
	if cfg.Timeout != 0 {
		t.Fatalf("unexpected timeout: %s", cfg.Timeout)
	}
}

func TestCheckResultAppliesOptions(t *testing.T) {
	var sizes []int
	generator := pbt.NewGenerator("size probe", func(_ *rand.Rand, size int) int {
		sizes = append(sizes, size)
		return 1
	})
	property := pbt.ForAll(
		"option application",
		generator,
		func(v int) bool {
			return v == 1
		},
	)

	result := pbt.CheckResult(
		property,
		pbt.WithRuns(7),
		pbt.WithMaxSize(9),
		pbt.WithSeed(1234),
		pbt.WithTimeout(50*time.Millisecond),
	)

	if !result.Passed {
		t.Fatalf("expected pass, got %s", result.Error())
	}
	if result.Runs != 7 {
		t.Fatalf("unexpected runs: %d", result.Runs)
	}
	if result.Seed != 1234 {
		t.Fatalf("unexpected seed: %d", result.Seed)
	}
	// The generator must observe exactly 7 sizes growing from 1 to MaxSize.
	if len(sizes) != 7 {
		t.Fatalf("expected 7 generated cases, got %d", len(sizes))
	}
	if sizes[0] != 1 || sizes[6] != 9 {
		t.Fatalf("unexpected size ramp: %v", sizes)
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] < sizes[i-1] {
			t.Fatalf("sizes not monotonic: %v", sizes)
		}
	}
}

func TestWithRunsPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for WithRuns(0)")
		}
	}()
	_ = pbt.CheckResult(
		pbt.ForAll("x", pbt.IntRange(1, 1), func(int) bool { return true }),
		pbt.WithRuns(0),
		pbt.WithSeed(1),
	)
}

func TestWithMaxSizePanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for WithMaxSize(-1)")
		}
	}()
	_ = pbt.CheckResult(
		pbt.ForAll("x", pbt.IntRange(1, 1), func(int) bool { return true }),
		pbt.WithMaxSize(-1),
		pbt.WithSeed(1),
	)
}
