//go:build debug
// +build debug

package tri

const debugMode = true

func debug(msg string, args ...any) {
	log.Infof(msg, args...)
}

func debugAssert(expr bool, msg string, args ...any) {
	if !expr {
		log.Errorf(msg, args...)
	}
}
