package indicatorv2

import (
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type FixedWindowVolumeProfile
type FixedWindowVolumeProfile struct {
	*types.Float64Series

	delta    float64 // price increment for each price level
	window   int     // number of KLines to aggregate before emitting a profile
	wCount   int
	accKLine types.KLine         // accumulated KLine for the current window
	profile  map[float64]float64 // price level -> volume

	useAvgOHLC                   bool
	minPriceLevel, maxPriceLevel float64
	resetCallbacks               []func(profile map[float64]float64, accKLine types.KLine)
}

func NewFixedWindowVolumeProfile(source KLineSubscription, window int, delta float64) *FixedWindowVolumeProfile {
	if delta <= 0 {
		panic("delta must be positive")
	}
	vp := &FixedWindowVolumeProfile{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		delta:         delta,
		profile:       make(map[float64]float64),
		minPriceLevel: math.Inf(1),
		maxPriceLevel: math.Inf(-1),
	}
	source.AddSubscriber(vp.calculateKline)
	return vp
}

func (vp *FixedWindowVolumeProfile) UseAvgOHLC() {
	vp.useAvgOHLC = true
}

func (vp *FixedWindowVolumeProfile) reset() {
	vp.wCount = 0
	vp.accKLine = types.KLine{}
	vp.profile = make(map[float64]float64)
	vp.minPriceLevel = math.Inf(1)
	vp.maxPriceLevel = math.Inf(-1)
}

func (vp *FixedWindowVolumeProfile) calculateKline(kline types.KLine) {
	if kline.Volume.IsZero() {
		return
	}

	var price float64
	if vp.useAvgOHLC {
		price = fixedpoint.Sum([]fixedpoint.Value{
			kline.Open,
			kline.High,
			kline.Low,
			kline.Close,
		}).Div(fixedpoint.Four).Float64()
	} else {
		price = kline.Close.Float64()
	}
	if vp.wCount == 0 {
		vp.accKLine.Set(&kline)
	} else {
		vp.accKLine.Merge(&kline)
	}

	priceLevel := math.Round(price / vp.delta)
	if vp.minPriceLevel > priceLevel {
		vp.minPriceLevel = priceLevel
	}
	if vp.maxPriceLevel < priceLevel {
		vp.maxPriceLevel = priceLevel
	}
	vp.profile[priceLevel] += kline.Volume.Float64()

	vp.wCount++
	if vp.wCount == vp.window {
		poc, _ := vp.PointOfControl()
		vp.PushAndEmit(poc)
		vp.EmitReset(vp.profile, vp.accKLine)
		vp.reset()
	}
}

func (vp *FixedWindowVolumeProfile) PointOfControl() (pocPrice, pocVolume float64) {
	var pocPriceLevel float64
	for priceLevel, volume := range vp.profile {
		if volume > pocVolume {
			pocPriceLevel = priceLevel
			pocVolume = volume
		}
	}
	return pocPriceLevel * vp.delta, pocVolume
}

func (vp *FixedWindowVolumeProfile) PointOfControlAboveEqual(priceLevel float64, limit ...float64) (pocPrice, pocVolume float64) {
	filter := vp.maxPriceLevel
	if len(limit) > 0 {
		filter = limit[0]
	}
	start := priceLevel
	if start > filter {
		return 0, 0
	}
	var pocPriceLevel float64
	for priceLevel, volume := range vp.profile {
		if priceLevel >= start && priceLevel <= filter && volume > pocVolume {
			pocPriceLevel = priceLevel
			pocVolume = volume
		}
	}
	return pocPriceLevel * vp.delta, pocVolume
}

func (vp *FixedWindowVolumeProfile) PointOfControlBelowEqual(priceLevel float64, limit ...float64) (pocPrice, pocVolume float64) {
	filter := vp.minPriceLevel
	if len(limit) > 0 {
		filter = limit[0]
	}
	start := priceLevel
	if start < filter {
		return 0, 0
	}
	var pocPriceLevel float64
	for priceLevel, volume := range vp.profile {
		if priceLevel <= start && priceLevel >= filter && volume > pocVolume {
			pocPriceLevel = priceLevel
			pocVolume = volume
		}
	}
	return pocPriceLevel * vp.delta, pocVolume
}
