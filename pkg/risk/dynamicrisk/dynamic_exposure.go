package dynamicrisk

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"math"
)

type DynamicExposure struct {
	// BollBandExposure calculates the max exposure with the Bollinger Band
	BollBandExposure *DynamicExposureBollBand `json:"bollBandExposure"`
}

// Initialize dynamic exposure
func (d *DynamicExposure) Initialize(symbol string, session *bbgo.ExchangeSession) {
	switch {
	case d.BollBandExposure != nil:
		d.BollBandExposure.initialize(symbol, session)
	}
}

func (d *DynamicExposure) IsEnabled() bool {
	return d.BollBandExposure != nil
}

// GetMaxExposure returns the max exposure
func (d *DynamicExposure) GetMaxExposure(price float64, trend types.Direction) (maxExposure fixedpoint.Value, err error) {
	switch {
	case d.BollBandExposure != nil:
		return d.BollBandExposure.getMaxExposure(price, trend)
	default:
		return fixedpoint.Zero, errors.New("dynamic exposure is not enabled")
	}
}

// DynamicExposureBollBand calculates the max exposure with the Bollinger Band
type DynamicExposureBollBand struct {
	// DynamicExposureBollBandScale is used to define the exposure range with the given percentage.
	DynamicExposureBollBandScale *bbgo.PercentageScale `json:"dynamicExposurePositionScale"`

	types.IntervalWindowBandWidth

	dynamicExposureBollBand *indicator.BOLL
}

// initialize dynamic exposure with Bollinger Band
func (d *DynamicExposureBollBand) initialize(symbol string, session *bbgo.ExchangeSession) {
	d.dynamicExposureBollBand = session.StandardIndicatorSet(symbol).BOLL(d.IntervalWindow, d.BandWidth)

	// Subscribe kline
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{
		Interval: d.dynamicExposureBollBand.Interval,
	})
}

// getMaxExposure returns the max exposure
func (d *DynamicExposureBollBand) getMaxExposure(price float64, trend types.Direction) (fixedpoint.Value, error) {
	downBand := d.dynamicExposureBollBand.DownBand.Last(0)
	upBand := d.dynamicExposureBollBand.UpBand.Last(0)
	sma := d.dynamicExposureBollBand.SMA.Last(0)
	log.Infof("dynamicExposureBollBand bollinger band: up %f sma %f down %f", upBand, sma, downBand)

	bandPercentage := 0.0
	if price < sma {
		// should be negative percentage
		bandPercentage = (price - sma) / math.Abs(sma-downBand)
	} else if price > sma {
		// should be positive percentage
		bandPercentage = (price - sma) / math.Abs(upBand-sma)
	}

	// Reverse if downtrend
	if trend == types.DirectionDown {
		bandPercentage = 0 - bandPercentage
	}

	v, err := d.DynamicExposureBollBandScale.Scale(bandPercentage)
	if err != nil {
		return fixedpoint.Zero, err
	}
	return fixedpoint.NewFromFloat(v), nil
}
