// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"maps"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func TestClassifiersAndCoverageAreRecorded(t *testing.T) {
	property := pbt.ForAll(
		"classifier and coverage",
		pbt.IntRange(-5, 5),
		func(v int) bool {
			return v != 3
		},
		pbt.WithClassifier(func(v int) []string {
			if v >= 0 {
				return []string{"non-negative"}
			}
			return []string{"negative"}
		}),
		pbt.WithLabeler(func(v int) []string {
			if v%2 == 0 {
				return []string{"even"}
			}
			return []string{"odd"}
		}),
		pbt.WithBucketer(func(v int) string {
			if v < 0 {
				return "lt0"
			}
			return "gte0"
		}),
	)

	result := pbt.CheckResult(property, pbt.WithSeed(3), pbt.WithRuns(100))
	if result.Passed {
		t.Fatalf("expected failing result")
	}
	if !result.HasCounterexample {
		t.Fatalf("expected counterexample")
	}
	// The only failing input is v == 3, so the classifier output is fixed.
	if len(result.FailureLabels) != 1 || result.FailureLabels[0] != "non-negative" {
		t.Fatalf("unexpected failure labels: %v", result.FailureLabels)
	}
	// The run stops at the first failure, so coverage counts must cover
	// exactly FailureIndex+1 generated cases.
	labelTotal := 0
	for key, count := range result.LabelCounts {
		if key != "even" && key != "odd" {
			t.Fatalf("unexpected label key %q", key)
		}
		labelTotal += count
	}
	if labelTotal != result.FailureIndex+1 {
		t.Fatalf("label counts cover %d cases, expected %d", labelTotal, result.FailureIndex+1)
	}
	bucketTotal := 0
	for key, count := range result.BucketCounts {
		if key != "lt0" && key != "gte0" {
			t.Fatalf("unexpected bucket key %q", key)
		}
		bucketTotal += count
	}
	if bucketTotal != result.FailureIndex+1 {
		t.Fatalf("bucket counts cover %d cases, expected %d", bucketTotal, result.FailureIndex+1)
	}
}

func TestParallelDeterministicPartitioning(t *testing.T) {
	property := pbt.ForAll(
		"parallel deterministic",
		pbt.IntRange(-50, 50),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	left := pbt.CheckResult(property, pbt.WithSeed(99), pbt.WithRuns(500), pbt.WithParallelism(4))
	right := pbt.CheckResult(property, pbt.WithSeed(99), pbt.WithRuns(500), pbt.WithParallelism(4))

	if left.Passed {
		t.Fatalf("expected the property to fail so determinism is exercised")
	}
	if left.Passed != right.Passed {
		t.Fatalf("pass mismatch")
	}
	if left.Counterexample != right.Counterexample {
		t.Fatalf("counterexample mismatch")
	}
	if left.FailureIndex != right.FailureIndex {
		t.Fatalf("failure index mismatch: %d vs %d", left.FailureIndex, right.FailureIndex)
	}
	if !reflect.DeepEqual(left.ShrinkTrace, right.ShrinkTrace) {
		t.Fatalf("shrink trace mismatch: %v vs %v", left.ShrinkTrace, right.ShrinkTrace)
	}
	if !maps.Equal(left.LabelCounts, right.LabelCounts) {
		t.Fatalf("label counts mismatch: %v vs %v", left.LabelCounts, right.LabelCounts)
	}
	if !maps.Equal(left.BucketCounts, right.BucketCounts) {
		t.Fatalf("bucket counts mismatch: %v vs %v", left.BucketCounts, right.BucketCounts)
	}
}

func TestSliceShrinkerInProperty(t *testing.T) {
	property := pbt.ForAll(
		"slice length under 3",
		pbt.SliceOf(pbt.IntRange(1, 10), 1, 8),
		func(s []int) bool { return len(s) < 3 },
		pbt.WithShrinker[[]int](pbt.SliceShrinker[int]()),
	)
	result := pbt.CheckResult(property, pbt.WithRuns(200), pbt.WithSeed(19))
	if result.Passed {
		t.Fatalf("expected failing property")
	}
	if !result.HasCounterexample {
		t.Fatalf("expected counterexample")
	}
	if len(result.Counterexample) < 3 {
		t.Fatalf("counterexample should have len>=3 to fail predicate, got %v", result.Counterexample)
	}
	if len(result.ShrinkTrace) == 0 && len(result.Counterexample) > 3 {
		t.Fatalf("expected shrink trace when counterexample is large")
	}
}

func TestSliceShrinkerProducesTrace(t *testing.T) {
	shrinker := pbt.SliceShrinker[int]()
	traced, ok := shrinker.(pbt.TraceShrinker[[]int])
	if !ok {
		t.Fatalf("SliceShrinker should implement TraceShrinker")
	}
	predicate := func(s []int) bool { return len(s) <= 2 }
	trace, final, changed := traced.ShrinkTrace([]int{1, 2, 3, 4, 5}, predicate)
	if !changed {
		t.Fatalf("expected shrink to change")
	}
	if len(trace) < 2 {
		t.Fatalf("expected trace with at least 2 steps, got %d", len(trace))
	}
	if trace[0][0] != 1 || len(trace[0]) != 5 {
		t.Fatalf("trace should start with original value")
	}
	if len(final) >= 5 {
		t.Fatalf("final should be shorter than original")
	}
}

func TestSuchThatInProperty(t *testing.T) {
	property := pbt.ForAll(
		"positive even ints are divisible by 2",
		pbt.SuchThat("positive even", pbt.IntRange(0, 100), func(v int) bool { return v > 0 && v%2 == 0 }, 200),
		func(v int) bool { return v%2 == 0 },
	)
	result := pbt.CheckResult(property, pbt.WithRuns(50), pbt.WithSeed(31))
	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Error())
	}
}

func TestSuchThatFallbackInProperty(t *testing.T) {
	fallback := 2
	property := pbt.ForAll(
		"even ints from SuchThatFallback",
		pbt.SuchThatFallback("even", pbt.IntRange(0, 100), func(v int) bool { return v%2 == 0 }, fallback, 200),
		func(v int) bool { return v%2 == 0 },
	)
	result := pbt.CheckResult(property, pbt.WithRuns(50), pbt.WithSeed(31))
	if !result.Passed {
		t.Fatalf("expected pass: %s", result.Error())
	}
}

func TestCombinatorsProduceValues(t *testing.T) {
	rng := rand.New(rand.NewSource(77))

	pair := pbt.Tuple2("pair", pbt.IntRange(1, 2), pbt.Bool())
	sawTrue, sawFalse := false, false
	for range 200 {
		pv := pair.Generate(rng, 10)
		if pv.First < 1 || pv.First > 2 {
			t.Fatalf("unexpected tuple first: %d", pv.First)
		}
		sawTrue = sawTrue || pv.Second
		sawFalse = sawFalse || !pv.Second
	}
	if !sawTrue || !sawFalse {
		t.Fatalf("Tuple2 second component did not vary: true=%v false=%v", sawTrue, sawFalse)
	}

	oneOf := pbt.OneOf("oneof", pbt.IntRange(1, 1), pbt.IntRange(2, 2))
	sawOne, sawTwo := false, false
	for range 200 {
		v := oneOf.Generate(rng, 10)
		if v != 1 && v != 2 {
			t.Fatalf("unexpected oneof value: %d", v)
		}
		sawOne = sawOne || v == 1
		sawTwo = sawTwo || v == 2
	}
	if !sawOne || !sawTwo {
		t.Fatalf("OneOf never selected a branch: one=%v two=%v", sawOne, sawTwo)
	}

	freq := pbt.Frequency("freq",
		pbt.WeightedGenerator[int]{Weight: 1, Generator: pbt.IntRange(1, 1)},
		pbt.WeightedGenerator[int]{Weight: 3, Generator: pbt.IntRange(2, 2)},
	)
	lowWeightHits, highWeightHits := 0, 0
	for range 400 {
		fv := freq.Generate(rng, 10)
		switch fv {
		case 1:
			lowWeightHits++
		case 2:
			highWeightHits++
		default:
			t.Fatalf("unexpected frequency value: %d", fv)
		}
	}
	if lowWeightHits == 0 || highWeightHits == 0 {
		t.Fatalf("Frequency never selected a branch: low=%d high=%d", lowWeightHits, highWeightHits)
	}
	if highWeightHits <= lowWeightHits {
		t.Fatalf("Frequency ignored weights: low=%d high=%d", lowWeightHits, highWeightHits)
	}

	triple := pbt.Tuple3("triple", pbt.IntRange(3, 3), pbt.IntRange(4, 5), pbt.Bool())
	for range 50 {
		tv := triple.Generate(rng, 5)
		if tv.First != 3 || tv.Second < 4 || tv.Second > 5 {
			t.Fatalf("unexpected triple value: %+v", tv)
		}
	}

	product := pbt.Product2("product", pbt.IntRange(9, 9), pbt.IntRange(8, 8))
	prv := product.Generate(rng, 5)
	if prv.First != 9 || prv.Second != 8 {
		t.Fatalf("unexpected product value: %+v", prv)
	}
}

func TestDistributionValidation(t *testing.T) {
	report := pbt.AnalyzeDistribution(
		pbt.Bool(),
		func(v bool) string {
			if v {
				return "true"
			}
			return "false"
		},
		2000,
		42,
		16,
	)

	err := pbt.ValidateDistribution(report, []pbt.DistributionRule{
		{Bucket: "true", MinPercent: 40, MaxPercent: 60},
		{Bucket: "false", MinPercent: 40, MaxPercent: 60},
	})
	if err != nil {
		t.Fatalf("unexpected distribution validation failure: %v", err)
	}
}

func TestReplayFixtureRoundTrip(t *testing.T) {
	property := pbt.ForAll(
		"fixture roundtrip",
		pbt.IntRange(1, 1),
		func(v int) bool { return v == 0 },
	)

	result := pbt.CheckResult(property, pbt.WithSeed(17), pbt.WithRuns(1))
	if result.Passed {
		t.Fatalf("expected failure")
	}

	fixture, err := result.ToReplayFixture()
	if err != nil {
		t.Fatalf("unexpected fixture conversion error: %v", err)
	}

	tmp := filepath.Join(t.TempDir(), "replay.json")
	if err := pbt.WriteReplayFixture(tmp, fixture); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	readBack, err := pbt.ReadReplayFixture(tmp)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if readBack.PropertyName != fixture.PropertyName {
		t.Fatalf("property mismatch")
	}
	if readBack.Seed != fixture.Seed {
		t.Fatalf("seed mismatch")
	}

	raw := string(readBack.Counterexample)
	val, err := pbt.DeserializeCounterexample[int](raw)
	if err != nil {
		t.Fatalf("unexpected deserialize error: %v", err)
	}
	if val != result.Counterexample {
		t.Fatalf("counterexample mismatch")
	}
}

type bankModel struct{}

func (bankModel) Name() string            { return "deposit" }
func (bankModel) Precondition(_ int) bool { return true }
func (bankModel) Next(r *rand.Rand, state int) int {
	return state + (r.Intn(3) + 1)
}

type withdrawModel struct{}

func (withdrawModel) Name() string                { return "withdraw" }
func (withdrawModel) Precondition(state int) bool { return state > 1 }
func (withdrawModel) Next(r *rand.Rand, state int) int {
	return state - (r.Intn(2) + 1)
}

func TestStatefulModelSupport(t *testing.T) {
	model := pbt.CommandModel[int]{
		Name: "counter model",
		Init: func(_ *rand.Rand) int { return 0 },
		Commands: []pbt.StatefulCommand[int]{
			bankModel{},
			withdrawModel{},
		},
		Invariant: func(state int) bool {
			return state >= 0
		},
	}

	result := pbt.CheckStatefulResult(model, pbt.WithSeed(5), pbt.WithRuns(100), pbt.WithMaxSize(50))
	if !result.Passed {
		t.Fatalf("expected model to pass: %s", result.Error())
	}
}

func TestShrinkTraceIsExposed(t *testing.T) {
	property := pbt.ForAll(
		"shrink trace",
		pbt.IntRange(64, 64),
		func(v int) bool { return v == 0 },
		pbt.WithShrinker[int](pbt.IntShrinker()),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(1), pbt.WithSeed(1))
	if result.Passed {
		t.Fatalf("expected failure")
	}
	if len(result.ShrinkTrace) == 0 {
		t.Fatalf("expected shrink trace to be populated")
	}
}

func TestCommandSequenceGenerator(t *testing.T) {
	gen := pbt.CommandSequence("cmd-seq", 2, 5, bankModel{}, withdrawModel{})
	rng := rand.New(rand.NewSource(8))
	allowed := map[string]bool{"deposit": true, "withdraw": true}
	sawMin, sawMax := false, false
	for range 200 {
		seq := gen.Generate(rng, 10)
		if len(seq) < 2 || len(seq) > 5 {
			t.Fatalf("unexpected sequence length: %d", len(seq))
		}
		for _, cmd := range seq {
			if !allowed[cmd.Name()] {
				t.Fatalf("sequence contains unknown command %q", cmd.Name())
			}
		}
		sawMin = sawMin || len(seq) == 2
		sawMax = sawMax || len(seq) == 5
	}
	if !sawMin || !sawMax {
		t.Fatalf("sequence lengths did not span bounds: min=%v max=%v", sawMin, sawMax)
	}

	// A size below minLen still honors the lower bound.
	short := gen.Generate(rng, 1)
	if len(short) != 2 {
		t.Fatalf("expected minLen to win over small size, got %d", len(short))
	}

	empty := pbt.CommandSequence[int]("empty", 0, 3)
	if got := empty.Generate(rng, 5); got != nil {
		t.Fatalf("expected nil sequence with no commands, got %v", got)
	}
}

func TestReplaySerializeCounterexampleStable(t *testing.T) {
	type pair struct {
		B int `json:"b"`
		A int `json:"a"`
	}
	payload, err := pbt.SerializeCounterexample(pair{B: 2, A: 1})
	if err != nil {
		t.Fatalf("unexpected serialize error: %v", err)
	}
	// Encoding follows struct field declaration order and must be byte-stable.
	if payload != `{"b":2,"a":1}` {
		t.Fatalf("unexpected payload: %s", payload)
	}
	again, err := pbt.SerializeCounterexample(pair{B: 2, A: 1})
	if err != nil {
		t.Fatalf("unexpected serialize error: %v", err)
	}
	if payload != again {
		t.Fatalf("serialization is not stable: %q vs %q", payload, again)
	}
	decoded, err := pbt.DeserializeCounterexample[pair](payload)
	if err != nil {
		t.Fatalf("unexpected deserialize error: %v", err)
	}
	if decoded.B != 2 || decoded.A != 1 {
		t.Fatalf("round-trip mismatch: %+v", decoded)
	}
}
