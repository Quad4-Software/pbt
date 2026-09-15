// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package pbt_test

import (
	"encoding/json"
	"math/rand"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func FuzzIntRangeWithinBounds(f *testing.F) {
	f.Add(0, 10, 1)
	f.Add(-100, 100, 42)
	f.Add(10, -10, 7)

	f.Fuzz(func(t *testing.T, low int, high int, seed int) {
		gen := pbt.IntRange(low, high)
		rng := rand.New(rand.NewSource(int64(seed)))

		lo, hi := low, high
		if lo > hi {
			lo, hi = hi, lo
		}

		for range 32 {
			v := gen.Generate(rng, 100)
			if v < lo || v > hi {
				t.Fatalf("value out of bounds: v=%d lo=%d hi=%d", v, lo, hi)
			}
		}
	})
}

func FuzzStringASCIIContract(f *testing.F) {
	f.Add(0, 16, 16, 1)
	f.Add(4, 64, 64, 99)
	f.Add(64, 4, 64, 99)

	alphabet := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	f.Fuzz(func(t *testing.T, low int, high int, size int, seed int) {
		nMin := normalize(low, 0, 128)
		nMax := normalize(high, 0, 128)
		nSize := normalize(size, 1, 128)

		gen := pbt.StringASCII(nMin, nMax)
		rng := rand.New(rand.NewSource(int64(seed)))
		out := gen.Generate(rng, nSize)

		lo, hi := nMin, nMax
		if lo > hi {
			lo, hi = hi, lo
		}
		if len(out) < lo || len(out) > hi {
			t.Fatalf("invalid length: got=%d lo=%d hi=%d", len(out), lo, hi)
		}
		for _, r := range out {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("invalid rune generated: %q", r)
			}
		}
	})
}

func FuzzCheckResultDeterministic(f *testing.F) {
	f.Add(int64(1), 10, 16)
	f.Add(int64(2026), 50, 64)

	f.Fuzz(func(t *testing.T, seed int64, runs int, maxSize int) {
		nRuns := normalize(runs, 1, 64)
		nMaxSize := normalize(maxSize, 1, 128)

		property := pbt.ForAll(
			"fuzz deterministic check",
			pbt.IntRange(-200, 200),
			func(v int) bool { return v == 0 },
			pbt.WithShrinker[int](pbt.IntShrinker()),
		)

		left := pbt.CheckResult(property, pbt.WithSeed(seed), pbt.WithRuns(nRuns), pbt.WithMaxSize(nMaxSize))
		right := pbt.CheckResult(property, pbt.WithSeed(seed), pbt.WithRuns(nRuns), pbt.WithMaxSize(nMaxSize))

		if left.Passed != right.Passed {
			t.Fatalf("pass mismatch: %v != %v", left.Passed, right.Passed)
		}
		if left.Counterexample != right.Counterexample {
			t.Fatalf("counterexample mismatch: %d != %d", left.Counterexample, right.Counterexample)
		}
		if left.HasCounterexample != right.HasCounterexample {
			t.Fatalf("counterexample presence mismatch")
		}
	})
}

// FuzzIntShrinkerPreservesFailure fuzzes the core shrink contract: the
// shrunk value must still fail the predicate and sit on a halving boundary.
func FuzzIntShrinkerPreservesFailure(f *testing.F) {
	f.Add(100, 5)
	f.Add(-1024, -7)
	f.Add(1, -10)
	f.Add(-3, -7)

	shrinker := pbt.IntShrinker()
	f.Fuzz(func(t *testing.T, value int, threshold int) {
		predicate := func(v int) bool { return v <= threshold }
		if predicate(value) {
			t.Skip("input must fail the predicate")
		}

		shrunk, changed := shrinker.Shrink(value, predicate)
		if predicate(shrunk) {
			t.Fatalf("shrunk value %d passes predicate (threshold=%d)", shrunk, threshold)
		}
		if shrunk != 0 && !predicate(shrunk/2) {
			t.Fatalf("not on halving boundary: shrunk=%d next=%d", shrunk, shrunk/2)
		}
		if changed && shrunk == value {
			t.Fatalf("changed flag set but value unchanged: %d", shrunk)
		}
		if !changed && shrunk != value {
			t.Fatalf("unchanged flag but value moved: %d -> %d", value, shrunk)
		}
		if value > 0 && shrunk < 0 || value < 0 && shrunk > 0 {
			t.Fatalf("halving flipped sign: %d -> %d", value, shrunk)
		}
	})
}

// FuzzSliceShrinkerPreservesFailure fuzzes the slice shrink contract: the
// result must still fail and must have the minimal failing length.
func FuzzSliceShrinkerPreservesFailure(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5}, 2)
	f.Add([]byte{9}, 0)
	f.Add([]byte{0, 0, 0}, 1)

	shrinker := pbt.SliceShrinker[int]()
	f.Fuzz(func(t *testing.T, raw []byte, limit int) {
		limit = normalize(limit, 0, 64)
		value := make([]int, len(raw))
		for i, b := range raw {
			value[i] = int(b)
		}
		predicate := func(s []int) bool { return len(s) <= limit }
		if predicate(value) {
			t.Skip("input must fail the predicate")
		}

		shrunk, changed := shrinker.Shrink(value, predicate)
		if predicate(shrunk) {
			t.Fatalf("shrunk slice len=%d passes predicate (limit=%d)", len(shrunk), limit)
		}
		if len(shrunk) != limit+1 {
			t.Fatalf("expected minimal failing length %d, got %d", limit+1, len(shrunk))
		}
		if len(value) > limit+1 && !changed {
			t.Fatalf("expected changed flag for %d -> %d", len(value), len(shrunk))
		}
		if !fuzzSubsequence(shrunk, value) {
			t.Fatalf("shrunk slice %v is not a subsequence of %v", shrunk, value)
		}
	})
}

func fuzzSubsequence(candidate []int, value []int) bool {
	i := 0
	for _, v := range value {
		if i < len(candidate) && candidate[i] == v {
			i++
		}
	}
	return i == len(candidate)
}

// FuzzReplayFixtureRoundTrip fuzzes fixture serialization: a fixture written
// to disk must read back field-for-field identical.
func FuzzReplayFixtureRoundTrip(f *testing.F) {
	f.Add("prop", int64(42), "gen", 7)
	f.Add("", int64(-1), "", -99)
	f.Add("weird \"name\"\n", int64(0), "g", 0)

	f.Fuzz(func(t *testing.T, name string, seed int64, genName string, ce int) {
		// JSON replaces invalid UTF-8 with U+FFFD on marshal, so the
		// round-trip contract only covers valid UTF-8 fields.
		if !utf8.ValidString(name) || !utf8.ValidString(genName) {
			t.Skip("fixture fields require valid UTF-8")
		}
		fixture := pbt.ReplayFixture{
			PropertyName:   name,
			Seed:           seed,
			GeneratorName:  genName,
			Counterexample: json.RawMessage(strconv.Itoa(ce)),
			FailureLabels:  []string{"a", "b"},
		}
		path := filepath.Join(t.TempDir(), "fixture.json")
		if err := pbt.WriteReplayFixture(path, fixture); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		back, err := pbt.ReadReplayFixture(path)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if back.PropertyName != name || back.Seed != seed || back.GeneratorName != genName {
			t.Fatalf("fixture fields corrupted: %+v", back)
		}
		if string(back.Counterexample) != strconv.Itoa(ce) {
			t.Fatalf("counterexample corrupted: %s", back.Counterexample)
		}
		if !slices.Equal(back.FailureLabels, fixture.FailureLabels) {
			t.Fatalf("labels corrupted: %v", back.FailureLabels)
		}
	})
}

func normalize(v int, lo int, hi int) int {
	if lo > hi {
		lo, hi = hi, lo
	}
	span := hi - lo + 1
	if span <= 0 {
		return lo
	}
	// #nosec G115 -- bit-pattern modulo keeps the result inside [lo, hi] for
	// any int64 input, including negative and extreme fuzz values.
	return lo + int(uint64(v)%uint64(span))
}
