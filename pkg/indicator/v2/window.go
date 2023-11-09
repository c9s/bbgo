package indicatorv2

// Window returns a copied slice of series starting at n index ago.
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
// Semantics of n argument are the same as Window function.
func WindowAppend[T any](series []T, n int, v T) []T {
	return Window(append(series, v), n)
}
