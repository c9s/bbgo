package indicatorv2

import "github.com/c9s/bbgo/pkg/types"

// The Average Directional Index (ADX), Minus Directional
// Indicator (-DI) and Plus Directional Indicator (+DI)
// represent a group of directional movement indicators
// that form a trading system developed by Welles Wilder.
// Although Wilder designed his Directional Movement System
// with commodities and daily prices in mind, these indicators
// can also be applied to stocks.
//
// Positive and negative directional movement form the backbone
// of the Directional Movement System. Wilder determined
// directional movement by comparing the difference between
// two consecutive lows with the difference between their
// respective highs.
//
// The Plus Directional Indicator (+DI) and Minus Directional
// Indicator (-DI) are derived from smoothed averages of
// these differences and measure trend direction over time.
// These two indicators are often collectively referred to
// as the Directional Movement Indicator (DMI).
//
// The Average Directional Index (ADX) is in turn derived
// from the smoothed averages of the difference between +DI
// and -DI; it measures the strength of the trend
// (regardless of direction) over time.
//
// Using these three indicators together, chartists can
// determine both the direction and strength of the trend.
//
//	https://school.stockcharts.com/doku.php?id=technical_indicators:average_directional_index_adx
//	https://www.investopedia.com/articles/trading/07/adx-trend-indicator.asp
//	https://www.fidelity.com/learning-center/trading-investing/technical-analysis/technical-indicator-guide/adx
type AdxStream struct {
	*types.Float64Series
	window int
	dx     *DxStream
	adx    *EWMAStream
}

func ADX(source KLineSubscription, window int) *AdxStream {
	s := &AdxStream{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		dx:            DirectionalIndex(source, window),
		adx:           EWMA2(nil, window, 1.0/float64(window)),
	}
	source.AddSubscriber(func(kLine types.KLine) {
		adx := s.adx.Calculate(s.dx.Last(0))
		s.adx.Push(adx)
		s.PushAndEmit(adx)
	})
	return s
}
