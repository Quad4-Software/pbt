// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

// recoverValue runs fn and returns the recovered panic value, failing the
// test when fn does not panic.
func recoverValue(t *testing.T, fn func()) (recovered any) {
	t.Helper()
	defer func() {
		recovered = recover()
		if recovered == nil {
			t.Fatalf("expected panic")
		}
	}()
	fn()
	return nil
}

func TestPredicatePanicPropagatesSequential(t *testing.T) {
	property := pbt.ForAll(
		"panicking predicate",
		pbt.IntRange(3, 3),
		func(v int) bool {
			if v == 3 {
				panic("kaboom")
			}
			return true
		},
	)
	got := recoverValue(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(5), pbt.WithSeed(1))
	})
	if got != "kaboom" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestPredicatePanicPropagatesParallel(t *testing.T) {
	// Without worker-side recovery this panic would escape a worker goroutine
	// and crash the whole test process instead of surfacing here.
	property := pbt.ForAll(
		"panicking predicate parallel",
		pbt.IntRange(3, 3),
		func(v int) bool {
			if v == 3 {
				panic("kaboom")
			}
			return true
		},
	)
	got := recoverValue(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(50), pbt.WithSeed(1), pbt.WithParallelism(4))
	})
	if got != "kaboom" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestParallelPanicIsDeterministic(t *testing.T) {
	// Every worker panics on a different generated value; the re-raised panic
	// must be identical across repeated runs.
	property := pbt.ForAll(
		"panic determinism",
		pbt.IntRange(10, 20),
		func(v int) bool {
			panic(fmt.Sprintf("panic-at-%d", v))
		},
	)

	var first any
	for i := range 3 {
		got := recoverValue(t, func() {
			_ = pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(9), pbt.WithParallelism(4))
		})
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("panic not deterministic: %v vs %v", first, got)
		}
	}
}

func TestGeneratorPanicPropagatesParallel(t *testing.T) {
	gen := pbt.NewGenerator("exploding", func(_ *rand.Rand, _ int) int {
		panic("generator exploded")
	})
	property := pbt.ForAll("gen panic", gen, func(_ int) bool { return true })
	got := recoverValue(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(20), pbt.WithSeed(2), pbt.WithParallelism(4))
	})
	if got != "generator exploded" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestLabelerPanicPropagates(t *testing.T) {
	property := pbt.ForAll(
		"labeler panic",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
		pbt.WithLabeler(func(_ int) []string { panic("labeler exploded") }),
	)
	got := recoverValue(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(3), pbt.WithSeed(1))
	})
	if got != "labeler exploded" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestPredicatePanicDuringShrinkPropagates(t *testing.T) {
	// The predicate fails for positive values and panics at zero, which the
	// int shrinker drives toward. The panic must propagate, not be swallowed.
	property := pbt.ForAll(
		"shrink panic",
		pbt.IntRange(4, 4),
		func(v int) bool {
			if v == 0 {
				panic("shrink hit zero")
			}
			return v > 5
		},
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)
	got := recoverValue(t, func() {
		_ = pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(1))
	})
	if got != "shrink hit zero" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestStatefulInvariantPanicPropagates(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "panic invariant",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			addCommand{},
		},
		Invariant: func(state int) bool {
			if state >= 2 {
				panic("invariant exploded")
			}
			return true
		},
	}
	got := recoverValue(t, func() {
		_ = pbt.CheckStatefulResult(model, pbt.WithRuns(1), pbt.WithMaxSize(4), pbt.WithSeed(1))
	})
	if got != "invariant exploded" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

func TestStatefulCommandPanicPropagates(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "panic command",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			panicCommand{},
		},
		Invariant: func(_ int) bool { return true },
	}
	got := recoverValue(t, func() {
		_ = pbt.CheckStatefulResult(model, pbt.WithRuns(1), pbt.WithMaxSize(4), pbt.WithSeed(1))
	})
	if got != "command exploded" {
		t.Fatalf("unexpected panic value: %v", got)
	}
}

type panicCommand struct{}

func (panicCommand) Name() string            { return "explode" }
func (panicCommand) Precondition(_ int) bool { return true }
func (panicCommand) Next(_ *rand.Rand, state int) int {
	panic("command exploded")
}
