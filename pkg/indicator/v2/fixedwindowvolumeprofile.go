package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type FixedWindowVolumeProfile
type FixedWindowVolumeProfile struct {
	*types.Float64Series

	delta    fixedpoint.Value // price increment for each price level
	window   int              // number of KLines to aggregate before emitting a profile
	wCount   int
	accKLine types.KLine                           // accumulated KLine for the current window
	profile  map[fixedpoint.Value]fixedpoint.Value // price level -> volume

	useAvgOHLC                   bool
	minPriceLevel, maxPriceLevel fixedpoint.Value
	resetCallbacks               []func(profile map[fixedpoint.Value]fixedpoint.Value, accKLine types.KLine)
}

func NewFixedWindowVolumeProfile(source KLineSubscription, window int, delta fixedpoint.Value) *FixedWindowVolumeProfile {
	if delta.Sign() <= 0 {
		panic("delta must be positive")
	}
	vp := &FixedWindowVolumeProfile{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		delta:         delta,
		profile:       make(map[fixedpoint.Value]fixedpoint.Value),
		minPriceLevel: fixedpoint.PosInf,
		maxPriceLevel: fixedpoint.NegInf,
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
	vp.profile = make(map[fixedpoint.Value]fixedpoint.Value)
	vp.minPriceLevel = fixedpoint.PosInf
	vp.maxPriceLevel = fixedpoint.NegInf
}

func (vp *FixedWindowVolumeProfile) calculateKline(kline types.KLine) {
	if kline.Volume.IsZero() {
		return
	}

	var price fixedpoint.Value
	if vp.useAvgOHLC {
		price = fixedpoint.Sum([]fixedpoint.Value{
			kline.Open,
			kline.High,
			kline.Low,
			kline.Close,
		}).Div(fixedpoint.Four)
	} else {
		price = kline.Close
	}
	if vp.wCount == 0 {
		vp.accKLine.Set(&kline)
	} else {
		vp.accKLine.Merge(&kline)
	}

	priceLevel := price.Div(vp.delta).Round(0, fixedpoint.HalfUp)
	if vp.minPriceLevel.Compare(priceLevel) > 0 {
		vp.minPriceLevel = priceLevel
	}
	if vp.maxPriceLevel.Compare(priceLevel) < 0 {
		vp.maxPriceLevel = priceLevel
	}
	vp.profile[priceLevel] = vp.profile[priceLevel].Add(kline.Volume)

	vp.wCount++
	if vp.wCount == vp.window {
		poc, _ := vp.PointOfControl()
		vp.PushAndEmit(poc.Float64())
		vp.EmitReset(vp.profile, vp.accKLine)
		vp.reset()
	}
}

func (vp *FixedWindowVolumeProfile) PointOfControl() (pocPrice, pocVolume fixedpoint.Value) {
	var pocPriceLevel fixedpoint.Value
	for priceLevel, volume := range vp.profile {
		if volume.Compare(pocVolume) > 0 {
			pocPriceLevel = priceLevel
			pocVolume = volume
		}
	}
	return pocPriceLevel.Mul(vp.delta), pocVolume
}

func (vp *FixedWindowVolumeProfile) PointOfControlAboveEqual(priceLevel fixedpoint.Value, limit ...fixedpoint.Value) (pocPrice, pocVolume fixedpoint.Value) {
	filter := vp.maxPriceLevel
	if len(limit) > 0 {
		filter = limit[0]
	}
	if priceLevel.Compare(filter) > 0 {
		return fixedpoint.Zero, fixedpoint.Zero
	}
	start := priceLevel

	var pocPriceLevel fixedpoint.Value
	for level, volume := range vp.profile {
		if level.Compare(start) >= 0 && level.Compare(filter) <= 0 && volume.Compare(pocVolume) > 0 {
			pocPriceLevel = level
			pocVolume = volume
		}
	}
	return pocPriceLevel.Mul(vp.delta), pocVolume
}

func (vp *FixedWindowVolumeProfile) PointOfControlBelowEqual(priceLevel fixedpoint.Value, limit ...fixedpoint.Value) (pocPrice, pocVolume fixedpoint.Value) {
	filter := vp.minPriceLevel
	if len(limit) > 0 {
		filter = limit[0]
	}
	if priceLevel.Compare(filter) < 0 {
		return fixedpoint.Zero, fixedpoint.Zero
	}
	start := priceLevel

	var pocPriceLevel fixedpoint.Value
	for level, volume := range vp.profile {
		if level.Compare(start) <= 0 && level.Compare(filter) >= 0 && volume.Compare(pocVolume) > 0 {
			pocPriceLevel = level
			pocVolume = volume
		}
	}
	return pocPriceLevel.Mul(vp.delta), pocVolume
}
