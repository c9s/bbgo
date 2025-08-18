//go:build !debug

package tri

const debugMode = false

func debug(msg string, args ...any) {}

func debugAssert(expr bool, msg string, args ...any) {}
