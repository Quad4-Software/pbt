// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestToReplayFixtureRequiresCounterexample(t *testing.T) {
	property := pbt.ForAll(
		"always passes",
		pbt.IntRange(0, 5),
		func(_ int) bool { return true },
	)
	result := pbt.CheckResult(property, pbt.WithRuns(10), pbt.WithSeed(1))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if _, err := result.ToReplayFixture(); err == nil {
		t.Fatalf("expected error converting a passing result")
	}
}

func TestReplayFixtureRoundTripPreservesAllFields(t *testing.T) {
	property := pbt.ForAll(
		"fixture fields",
		pbt.IntRange(50, 60),
		func(v int) bool { return v < 55 },
		pbt.WithClassifier(func(v int) []string { return []string{"too-big"} }),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(30), pbt.WithSeed(11))
	if result.Passed {
		t.Fatalf("expected failure")
	}

	fixture, err := result.ToReplayFixture()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fixture.PropertyName != "fixture fields" || fixture.GeneratorName != "IntRange" {
		t.Fatalf("unexpected fixture names: %+v", fixture)
	}
	if fixture.Seed != 11 {
		t.Fatalf("unexpected fixture seed: %d", fixture.Seed)
	}
	if !reflect.DeepEqual(fixture.FailureLabels, []string{"too-big"}) {
		t.Fatalf("unexpected fixture labels: %v", fixture.FailureLabels)
	}

	path := filepath.Join(t.TempDir(), "fx.json")
	if err := pbt.WriteReplayFixture(path, fixture); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	back, err := pbt.ReadReplayFixture(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !reflect.DeepEqual(back, fixture) {
		t.Fatalf("fixture round-trip mismatch:\n%+v\n%+v", back, fixture)
	}
}

func TestReadReplayFixtureErrors(t *testing.T) {
	if _, err := pbt.ReadReplayFixture(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("expected error for missing file")
	}

	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := pbt.ReadReplayFixture(bad); err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

func TestReplayFixtureValueErrors(t *testing.T) {
	property := pbt.ForAll("p", pbt.IntRange(0, 1), func(_ int) bool { return true })

	nilPredicate := pbt.Property[int]{Name: "np", Generator: pbt.IntRange(0, 1)}
	if _, err := pbt.ReplayFixtureValue(nilPredicate, pbt.ReplayFixture{Counterexample: json.RawMessage("1")}); err == nil {
		t.Fatalf("expected error for nil predicate")
	}
	if _, err := pbt.ReplayFixtureValue(property, pbt.ReplayFixture{}); err == nil {
		t.Fatalf("expected error for empty counterexample")
	}
	if _, err := pbt.ReplayFixtureValue(property, pbt.ReplayFixture{Counterexample: json.RawMessage("{oops")}); err == nil {
		t.Fatalf("expected error for malformed counterexample JSON")
	}
}

func TestReplayFixtureValueDetectsFixedPredicate(t *testing.T) {
	// The fixture records a value that used to fail. If the predicate is now
	// fixed, the replay must report StillFailing=false.
	property := pbt.ForAll(
		"fixed predicate",
		pbt.IntRange(0, 1),
		func(_ int) bool { return true },
	)
	fixture := pbt.ReplayFixture{
		PropertyName:   "fixed predicate",
		Seed:           3,
		GeneratorName:  "IntRange",
		Counterexample: json.RawMessage("42"),
	}
	replayed, err := pbt.ReplayFixtureValue(property, fixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replayed.StillFailing {
		t.Fatalf("expected StillFailing=false for a fixed predicate")
	}
	if replayed.Value != 42 {
		t.Fatalf("unexpected replayed value: %v", replayed.Value)
	}
	if !strings.Contains(replayed.Reason, "now passes") {
		t.Fatalf("unexpected reason: %q", replayed.Reason)
	}
}

func TestDeserializeCounterexampleBadPayload(t *testing.T) {
	if _, err := pbt.DeserializeCounterexample[int]("\"not an int\""); err == nil {
		t.Fatalf("expected deserialize error")
	}
	if _, err := pbt.DeserializeCounterexample[int]("{"); err == nil {
		t.Fatalf("expected deserialize error for truncated JSON")
	}
}

func TestSerializeCounterexampleUnsupportedType(t *testing.T) {
	if _, err := pbt.SerializeCounterexample(func() {}); err == nil {
		t.Fatalf("expected serialize error for func value")
	}
}

func TestReplayFixtureFileMissingFile(t *testing.T) {
	property := pbt.ForAll("p", pbt.IntRange(0, 1), func(_ int) bool { return true })
	if _, err := pbt.ReplayFixtureFile(property, filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatalf("expected error for missing fixture file")
	}
}

func TestStatefulReplayFixtureRoundTripPreservesSeeds(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "replay model",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			noopCommand{},
			addCommand{},
		},
		Invariant: func(state int) bool { return state < 3 },
	}

	var result pbt.StatefulResult[int]
	found := false
	for seed := int64(1); seed <= 100; seed++ {
		result = pbt.CheckStatefulResult(model, pbt.WithSeed(seed), pbt.WithRuns(1), pbt.WithMaxSize(32))
		if !result.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a failing seed")
	}

	fixture, err := result.ToStatefulReplayFixture()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "stateful.json")
	if err := pbt.WriteStatefulReplayFixture(path, fixture); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	back, err := pbt.ReadStatefulReplayFixture(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !reflect.DeepEqual(back, fixture) {
		t.Fatalf("stateful fixture round-trip mismatch:\n%+v\n%+v", back, fixture)
	}

	replayed, err := pbt.ReplayStatefulFixtureFile(model, path)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if replayed.Passed {
		t.Fatalf("expected replay to reproduce the failure")
	}
	if replayed.FailedCommand != result.FailedCommand {
		t.Fatalf("replay command mismatch: %q vs %q", replayed.FailedCommand, result.FailedCommand)
	}
	if !reflect.DeepEqual(replayed.Trace, result.Trace) {
		t.Fatalf("replay trace mismatch: %v vs %v", replayed.Trace, result.Trace)
	}
	if replayed.FinalState != result.FinalState {
		t.Fatalf("replay state mismatch: %v vs %v", replayed.FinalState, result.FinalState)
	}
}

func TestToStatefulReplayFixtureOnPassingResult(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name:      "ok model",
		Init:      func(_ *rand.Rand) int { return 0 },
		Commands:  []pbt.StatefulCommand[int]{addCommand{}},
		Invariant: func(_ int) bool { return true },
	}
	result := pbt.CheckStatefulResult(model, pbt.WithRuns(3), pbt.WithSeed(1))
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if _, err := result.ToStatefulReplayFixture(); err == nil {
		t.Fatalf("expected error for passing result")
	}
}

func TestReadStatefulReplayFixtureErrors(t *testing.T) {
	if _, err := pbt.ReadStatefulReplayFixture(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
