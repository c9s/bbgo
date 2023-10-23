// Copyright 2022 The Coln Group Ltd
// SPDX-License-Identifier: MIT

package indicatorv2

// Lookback returns a value from series at n index ago.
// Series must be in chronological order, with the earliest value at slice index 0.
// n = 0 returns the latest value. n = 1 returns the value before the latest etc.
func Lookback[T any](series []T, n int) T {
	var empty T
	i := (len(series) - n) - 1
	if i < 0 {
		return empty
	}
	return series[i]
}

// Window returns a copied slice of series starting at n index ago.
// Semantics of n argument are the same as Lookback function.
func Window[T any](series []T, n int) []T {

	ln := len(series)
	if ln <= n {
		window := make([]T, ln)
		copy(window, series)
		return window
	}

	i := (ln - n) - 1
	window := make([]T, n+1)
	copy(window, series[i:])

	return window
}

// WindowAppend appends a value to the end of the series and slices it to the window starting at n index ago.
// Semantics of n argument are the same as Window and Lookback functions.
func WindowAppend[T any](series []T, n int, v T) []T {
	return Window(append(series, v), n)
}
