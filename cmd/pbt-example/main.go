// SPDX-License-Identifier: 0BSD
// Copyright (c) 2026 Quad4
package main

import (
	"fmt"

	"github.com/Quad4-Software/pbt/pkg/pbt"
)

func main() {
	property := pbt.ForAll(
		"double reverse string preserves value",
		pbt.StringASCII(0, 32),
		func(in string) bool {
			return reverse(reverse(in)) == in
		},
		pbt.WithShrinker[string](pbt.StringShrinker()),
	)

	result := pbt.CheckResult(property, pbt.WithRuns(500), pbt.WithSeed(42))
	if result.Passed {
		fmt.Printf("PASS: property=%q runs=%d seed=%d\n", result.PropertyName, result.Runs, result.Seed)
		return
	}

	fmt.Printf("FAIL: %s\n", result.Error())
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
