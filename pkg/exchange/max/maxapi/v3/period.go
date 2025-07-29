package v3

import (
	"fmt"
	"strings"
)

type Period int64

func IntervalToPeriod(a string) (Period, error) {
	switch strings.ToLower(a) {

	case "1m":
		return 1, nil

	case "5m":
		return 5, nil

	case "15m":
		return 15, nil

	case "30m":
		return 30, nil

	case "1h":
		return 60, nil

	case "2h":
		return 60 * 2, nil

	case "3h":
		return 60 * 3, nil

	case "4h":
		return 60 * 4, nil

	case "6h":
		return 60 * 6, nil

	case "8h":
		return 60 * 8, nil

	case "12h":
		return 60 * 12, nil

	case "1d":
		return 60 * 24, nil

	case "3d":
		return 60 * 24 * 3, nil

	case "1w":
		return 60 * 24 * 7, nil

	}

	return 0, fmt.Errorf("incorrect resolution: %q", a)
}
