package bollmaker

import (
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
)

type PriceTrend string

const (
	NeutralTrend PriceTrend = "neutral"
	UpTrend      PriceTrend = "upTrend"
	DownTrend    PriceTrend = "downTrend"
	UnknownTrend PriceTrend = "unknown"
)

func detectPriceTrend(inc *indicatorv2.BOLLStream, price float64) PriceTrend {
	if inBetween(price, inc.DownBand.Last(0), inc.UpBand.Last(0)) {
		return NeutralTrend
	}

	if price < inc.DownBand.Last(0) {
		return DownTrend
	}

	if price > inc.UpBand.Last(0) {
		return UpTrend
	}

	return UnknownTrend
}
