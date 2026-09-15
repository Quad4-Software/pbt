// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt

import (
	"math"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"
)

func TestForAll2MatchesTuple2Oracle(t *testing.T) {
	// ForAll2 must observe exactly the same stream as ForAll over Tuple2.
	pair := ForAll("oracle", Tuple2("pair", Int(), Int()), func(v Tuple2Value[int, int]) bool {
		return v.First+v.Second < 1000
	})
	multi := ForAll2("subject", Int(), Int(), func(a, b int) bool {
		return a+b < 1000
	})

	opts := []Option{WithRuns(200), WithSeed(99)}
	want := CheckResult(pair, opts...)
	got := CheckResult(multi, opts...)

	if want.HasCounterexample != got.HasCounterexample {
		t.Fatalf("counterexample presence differs: oracle=%v subject=%v", want.HasCounterexample, got.HasCounterexample)
	}
	if got.Counterexample.First+got.Counterexample.Second < 1000 {
		t.Fatalf("counterexample does not violate the predicate: %+v", got.Counterexample)
	}
	if want.HasCounterexample && got.Counterexample != want.Counterexample {
		t.Fatalf("counterexample differs from tuple oracle: oracle=%+v subject=%+v", want.Counterexample, got.Counterexample)
	}
}

func TestForAll3EvaluatesAllArguments(t *testing.T) {
	seen := ForAll3("triple", IntRange(0, 5), Bool(), IntRange(-5, -1), func(a int, b bool, c int) bool {
		return a >= 0 && a <= 5 && c < 0
	})
	result := CheckResult(seen, WithRuns(100), WithSeed(7))
	if !result.Passed {
		t.Fatalf("expected pass, got %s", result.Error())
	}
}

func TestPreconditionOnlyEvaluatesMatchingCases(t *testing.T) {
	var evaluated []int
	property := ForAll("pos", IntRange(-100, 100), func(v int) bool {
		evaluated = append(evaluated, v)
		return true
	}, WithPrecondition(func(v int) bool { return v > 0 }))

	result := CheckResult(property, WithRuns(50), WithSeed(3))
	if !result.Passed {
		t.Fatalf("expected pass, got %s", result.Error())
	}
	if result.Skipped == 0 {
		t.Fatal("expected some generated cases to be skipped")
	}
	if len(evaluated) != 50 {
		t.Fatalf("expected 50 evaluated cases, got %d", len(evaluated))
	}
	for _, v := range evaluated {
		if v <= 0 {
			t.Fatalf("precondition rejected value %d reached the predicate", v)
		}
	}
}

func TestPreconditionExhaustionFails(t *testing.T) {
	property := ForAll("never", Int(), func(v int) bool { return true },
		WithPrecondition(func(v int) bool { return false }))

	result := CheckResult(property, WithRuns(10), WithMaxDiscards(20), WithSeed(1))
	if result.Passed {
		t.Fatal("expected exhaustion failure")
	}
	if !result.Exhausted {
		t.Fatalf("expected Exhausted result, got %+v", result)
	}
	if result.Skipped <= 20 {
		t.Fatalf("expected skipped to exceed the discard limit, got %d", result.Skipped)
	}
}

func TestPreconditionParallelDeterminism(t *testing.T) {
	property := ForAll("par", IntRange(-50, 50), func(v int) bool {
		return v < 40
	}, WithPrecondition(func(v int) bool { return v >= 0 }))

	opts := []Option{WithRuns(80), WithSeed(11), WithParallelism(4)}
	first := CheckResult(property, opts...)
	second := CheckResult(property, opts...)

	if first.Skipped != second.Skipped {
		t.Fatalf("skipped count not deterministic: %d vs %d", first.Skipped, second.Skipped)
	}
	if !reflect.DeepEqual(first.Counterexample, second.Counterexample) {
		t.Fatalf("counterexample not deterministic: %v vs %v", first.Counterexample, second.Counterexample)
	}
	if first.HasCounterexample && first.Counterexample < 40 {
		t.Fatalf("counterexample does not violate the predicate: %v", first.Counterexample)
	}
	if first.HasCounterexample && first.Counterexample < 0 {
		t.Fatalf("counterexample violates the precondition: %v", first.Counterexample)
	}
}

func TestFlatMapCorrelatesValues(t *testing.T) {
	// The length drawn first drives the slice length drawn second. An
	// independent Tuple2 could not express this.
	gen := FlatMap("sized", IntRange(0, 12), func(n int) Generator[[]int] {
		return SliceOf(Int(), n, n)
	})

	result := CheckResult(ForAll("correlated", gen, func(v []int) bool {
		return len(v) >= 0 && len(v) <= 12
	}), WithRuns(100), WithSeed(5))
	if !result.Passed {
		t.Fatalf("expected pass, got %s", result.Error())
	}

	// Verify the correlation itself, not just the bounds: the outer draw must
	// decide the inner length.
	samples := Sample(gen, 40, 5)
	seen := map[int]bool{}
	for _, s := range samples {
		seen[len(s)] = true
	}
	if len(seen) < 3 {
		t.Fatalf("flatmap produced too few distinct lengths: %v", seen)
	}
}

func TestSliceShrinkerOfShrinksElements(t *testing.T) {
	fails := func(v []int) bool {
		for _, e := range v {
			if e > 10 {
				return false
			}
		}
		return true
	}

	final, changed := SliceShrinkerOf(IntShrinker()).Shrink([]int{100, 20, 5}, fails)
	if !changed {
		t.Fatal("expected shrinker to modify the slice")
	}
	if fails(final) {
		t.Fatalf("shrinker produced a passing value: %v", final)
	}
	// Structural shrinking alone bottoms out at [100]; element shrinking must
	// reduce the surviving element further.
	if !reflect.DeepEqual(final, []int{12}) && len(final) > 1 {
		t.Fatalf("unexpected shrink result: %v", final)
	}
	for _, e := range final {
		if e >= 100 {
			t.Fatalf("element was not shrunk: %v", final)
		}
	}
}

func TestIntShrinkerToward(t *testing.T) {
	// Fails for every value except exactly 42; shrinking toward 42 must stop
	// at the smallest failing neighbor.
	failsUnless42 := func(v int) bool { return v == 42 }
	final, changed := IntShrinkerToward(42).Shrink(1000, failsUnless42)
	if !changed {
		t.Fatal("expected shrink")
	}
	if final != 43 {
		t.Fatalf("expected 43 (first failing value above target), got %d", final)
	}
}

func TestInt64AndUint64Shrinkers(t *testing.T) {
	fails := func(v int64) bool { return v <= 10 }
	final64, changed := Int64Shrinker().Shrink(1000, fails)
	if !changed || final64 > 15 || final64 < 11 {
		t.Fatalf("unexpected int64 shrink result: %d changed=%v", final64, changed)
	}

	ufails := func(v uint64) bool { return v <= 10 }
	ufinal, uchanged := Uint64Shrinker().Shrink(1000, ufails)
	if !uchanged || ufinal > 15 || ufinal < 11 {
		t.Fatalf("unexpected uint64 shrink result: %d changed=%v", ufinal, uchanged)
	}
}

func TestTuple2Shrinker(t *testing.T) {
	fails := func(v Tuple2Value[int, int]) bool { return v.First+v.Second <= 10 }
	final, changed := Tuple2Shrinker(IntShrinker(), IntShrinker()).Shrink(Tuple2Value[int, int]{First: 100, Second: 100}, fails)
	if !changed {
		t.Fatal("expected shrink")
	}
	if fails(final) {
		// The shrunk value must still violate the property (sum > 10).
		t.Fatalf("shrinker produced a passing value: %+v", final)
	}
	if final.First+final.Second >= 100 {
		t.Fatalf("tuple was not minimized: %+v", final)
	}
}

func TestNewGeneratorBounds(t *testing.T) {
	seed := int64(42)

	for _, v := range Sample(Int64Range(-1000, 1000), 200, seed) {
		if v < -1000 || v > 1000 {
			t.Fatalf("Int64Range out of bounds: %d", v)
		}
	}
	for _, b := range Sample(Bytes(3, 9), 200, seed) {
		if len(b) < 3 || len(b) > 9 {
			t.Fatalf("Bytes out of bounds: %d", len(b))
		}
	}
	for _, s := range Sample(String(0, 20), 200, seed) {
		if !utf8.ValidString(s) {
			t.Fatalf("String produced invalid UTF-8: %q", s)
		}
		if n := utf8.RuneCountInString(s); n > 20 {
			t.Fatalf("String too long: %d runes", n)
		}
	}
	for _, m := range Sample(MapOf(IntRange(0, 100), Bool(), 2, 10), 200, seed) {
		if len(m) > 10 {
			t.Fatalf("MapOf out of bounds: %d", len(m))
		}
	}
	for _, d := range Sample(DurationRange(time.Second, time.Minute), 200, seed) {
		if d < time.Second || d > time.Minute {
			t.Fatalf("DurationRange out of bounds: %s", d)
		}
	}
	for _, p := range Sample(PtrOf(Int(), 100), 20, seed) {
		if p != nil {
			t.Fatal("PtrOf with nilPercent=100 produced a non-nil value")
		}
	}
	for _, p := range Sample(PtrOf(Int(), 0), 20, seed) {
		if p == nil {
			t.Fatal("PtrOf with nilPercent=0 produced nil")
		}
	}
}

func TestSampleDeterministic(t *testing.T) {
	a := Sample(SliceOf(IntRange(0, 50), 0, 8), 50, 1234)
	b := Sample(SliceOf(IntRange(0, 50), 0, 8), 50, 1234)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed produced different samples")
	}
	if len(a) != 50 {
		t.Fatalf("expected 50 samples, got %d", len(a))
	}
}

func TestFullRangeGeneratorsDoNotPanic(t *testing.T) {
	Check(t, ForAll("int64", Int64(), func(v int64) bool { return v == v }), WithRuns(50))
	Check(t, ForAll("uint64", Uint64(), func(v uint64) bool { return v <= math.MaxUint64 }), WithRuns(50))
}

func TestSliceElemShrinkerTrace(t *testing.T) {
	fails := func(v []int) bool {
		for _, e := range v {
			if e > 10 {
				return false
			}
		}
		return true
	}

	shrinker, ok := SliceShrinkerOf(IntShrinker()).(TraceShrinker[[]int])
	if !ok {
		t.Fatal("SliceShrinkerOf must implement TraceShrinker")
	}
	trace, final, changed := shrinker.ShrinkTrace([]int{100, 20, 5}, fails)
	if !changed || len(trace) < 2 {
		t.Fatalf("expected a shrink trace, got %v changed=%v", trace, changed)
	}
	if fails(final) {
		t.Fatalf("trace ended on a passing value: %v", final)
	}
	for _, step := range trace {
		if len(step) > 3 {
			t.Fatalf("trace step grew the slice: %v", step)
		}
	}
}

func TestTuple3ShrinkerAndRemainingAPIs(t *testing.T) {
	fails := func(v Tuple3Value[int, int64, []byte]) bool {
		return int64(v.First)+v.Second+int64(len(v.Third)) <= 10
	}
	final, changed := Tuple3Shrinker(IntShrinker(), Int64Shrinker(), BytesShrinker()).Shrink(
		Tuple3Value[int, int64, []byte]{First: 100, Second: 100, Third: []byte{1, 2, 3, 4, 5}}, fails)
	if !changed {
		t.Fatal("expected shrink")
	}
	if fails(final) {
		t.Fatalf("shrinker produced a passing value: %+v", final)
	}
	if int64(final.First)+final.Second+int64(len(final.Third)) >= 100 {
		t.Fatalf("tuple was not minimized: %+v", final)
	}

	// Int64ShrinkerToward mirrors IntShrinkerToward.
	failsUnless42 := func(v int64) bool { return v == 42 }
	got, changed64 := Int64ShrinkerToward(42).Shrink(1000, failsUnless42)
	if !changed64 || got != 43 {
		t.Fatalf("expected 43, got %d changed=%v", got, changed64)
	}

	// BytesShrinker shortens while the failure holds.
	bfails := func(v []byte) bool { return len(v) < 2 }
	bfinal, bchanged := BytesShrinker().Shrink([]byte{9, 9, 9, 9}, bfails)
	if !bchanged || len(bfinal) >= 4 || len(bfinal) == 0 {
		t.Fatalf("unexpected bytes shrink: %v changed=%v", bfinal, bchanged)
	}

	// Float64 stays in [0,1).
	for _, f := range Sample(Float64(), 100, 9) {
		if f < 0 || f >= 1 || math.IsNaN(f) {
			t.Fatalf("Float64 out of range: %v", f)
		}
	}
}

func TestGeneratorInputNormalization(t *testing.T) {
	seed := int64(1)
	// Swapped bounds normalize instead of panicking.
	for _, v := range Sample(Int64Range(100, -100), 100, seed) {
		if v < -100 || v > 100 {
			t.Fatalf("normalized Int64Range out of bounds: %d", v)
		}
	}
	for _, v := range Sample(Bytes(-5, -1), 20, seed) {
		if len(v) < 0 {
			t.Fatalf("normalized Bytes negative length: %d", len(v))
		}
	}
	for _, s := range Sample(String(9, 2), 20, seed) {
		if utf8.RuneCountInString(s) > 9 {
			t.Fatalf("normalized String too long: %d", utf8.RuneCountInString(s))
		}
	}
	for _, d := range Sample(DurationRange(time.Hour, time.Minute), 20, seed) {
		if d < time.Minute || d > time.Hour {
			t.Fatalf("normalized DurationRange out of bounds: %s", d)
		}
	}
	// Out-of-range nilPercent clamps.
	for _, p := range Sample(PtrOf(Int(), -50), 20, seed) {
		if p == nil {
			t.Fatal("clamped nilPercent produced nil")
		}
	}
}
