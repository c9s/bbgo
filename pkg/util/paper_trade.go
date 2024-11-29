package util

import "github.com/c9s/bbgo/pkg/envvar"

func IsPaperTrade() bool {
	v, ok := envvar.Bool("PAPER_TRADE")
	return ok && v
}
