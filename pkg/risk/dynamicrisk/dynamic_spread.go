package dynamicrisk

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type DynamicSpread struct {
	// AmpSpread calculates spreads based on kline amplitude
	AmpSpread *DynamicAmpSpread `json:"amplitude"`

	// WeightedBollWidthRatioSpread calculates spreads based on two Bollinger Bands
	WeightedBollWidthRatioSpread *DynamicSpreadBollWidthRatio `json:"weightedBollWidth"`
}

// Initialize dynamic spread
func (ds *DynamicSpread) Initialize(symbol string, session *bbgo.ExchangeSession) {
	switch {
	case ds.AmpSpread != nil:
		ds.AmpSpread.initialize(symbol, session)
	case ds.WeightedBollWidthRatioSpread != nil:
		ds.WeightedBollWidthRatioSpread.initialize(symbol, session)
	}
}

func (ds *DynamicSpread) IsEnabled() bool {
	return ds.AmpSpread != nil || ds.WeightedBollWidthRatioSpread != nil
}

// GetAskSpread returns current ask spread
func (ds *DynamicSpread) GetAskSpread() (askSpread float64, err error) {
	switch {
	case ds.AmpSpread != nil:
		return ds.AmpSpread.getAskSpread()
	case ds.WeightedBollWidthRatioSpread != nil:
		return ds.WeightedBollWidthRatioSpread.getAskSpread()
	default:
		return 0, errors.New("dynamic spread is not enabled")
	}
}

// GetBidSpread returns current dynamic bid spread
func (ds *DynamicSpread) GetBidSpread() (bidSpread float64, err error) {
	switch {
	case ds.AmpSpread != nil:
		return ds.AmpSpread.getBidSpread()
	case ds.WeightedBollWidthRatioSpread != nil:
		return ds.WeightedBollWidthRatioSpread.getBidSpread()
	default:
		return 0, errors.New("dynamic spread is not enabled")
	}
}

// DynamicSpreadAmp uses kline amplitude to calculate spreads
type DynamicAmpSpread struct {
	types.IntervalWindow

	// AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`

	dynamicAskSpread *indicator.SMA
	dynamicBidSpread *indicator.SMA
}

// initialize amplitude dynamic spread and preload SMAs
func (ds *DynamicAmpSpread) initialize(symbol string, session *bbgo.ExchangeSession) {
	ds.dynamicBidSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}
	ds.dynamicAskSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}

	// Subscribe kline
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{
		Interval: ds.Interval,
	})

	// Update on kline closed
	session.MarketDataStream.OnKLineClosed(types.KLineWith(symbol, ds.Interval, func(kline types.KLine) {
		ds.update(kline)
	}))

	// Preload
	kLineStore, _ := session.MarketDataStore(symbol)
	if klines, ok := kLineStore.KLinesOfInterval(ds.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			ds.update((*klines)[i])
		}
	}
}

// update amplitude dynamic spread with kline
func (ds *DynamicAmpSpread) update(kline types.KLine) {
	// ampl is the amplitude of kline
	ampl := (kline.GetHigh().Float64() - kline.GetLow().Float64()) / kline.GetOpen().Float64()

	switch kline.Direction() {
	case types.DirectionUp:
		ds.dynamicAskSpread.Update(ampl)
		ds.dynamicBidSpread.Update(0)
	case types.DirectionDown:
		ds.dynamicBidSpread.Update(ampl)
		ds.dynamicAskSpread.Update(0)
	default:
		ds.dynamicAskSpread.Update(0)
		ds.dynamicBidSpread.Update(0)
	}
}

func (ds *DynamicAmpSpread) getAskSpread() (askSpread float64, err error) {
	if ds.AskSpreadScale != nil && ds.dynamicAskSpread.Length() >= ds.Window {
		askSpread, err = ds.AskSpreadScale.Scale(ds.dynamicAskSpread.Last(0))
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicAskSpread")
			return 0, err
		}

		return askSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}

func (ds *DynamicAmpSpread) getBidSpread() (bidSpread float64, err error) {
	if ds.BidSpreadScale != nil && ds.dynamicBidSpread.Length() >= ds.Window {
		bidSpread, err = ds.BidSpreadScale.Scale(ds.dynamicBidSpread.Last(0))
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicBidSpread")
			return 0, err
		}

		return bidSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}

type DynamicSpreadBollWidthRatio struct {
	// AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`

	// Sensitivity factor of the weighting function: 1 / (1 + exp(-(x - mid) * sensitivity / width))
	// A positive number. The greater factor, the sharper weighting function. Default set to 1.0 .
	Sensitivity float64 `json:"sensitivity"`

	DefaultBollinger types.IntervalWindowBandWidth `json:"defaultBollinger"`
	NeutralBollinger types.IntervalWindowBandWidth `json:"neutralBollinger"`

	neutralBoll *indicator.BOLL
	defaultBoll *indicator.BOLL
}

func (ds *DynamicSpreadBollWidthRatio) initialize(symbol string, session *bbgo.ExchangeSession) {
	ds.neutralBoll = session.StandardIndicatorSet(symbol).BOLL(ds.NeutralBollinger.IntervalWindow, ds.NeutralBollinger.BandWidth)
	ds.defaultBoll = session.StandardIndicatorSet(symbol).BOLL(ds.DefaultBollinger.IntervalWindow, ds.DefaultBollinger.BandWidth)

	// Subscribe kline
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{
		Interval: ds.NeutralBollinger.Interval,
	})
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{
		Interval: ds.DefaultBollinger.Interval,
	})

	if ds.Sensitivity <= 0. {
		ds.Sensitivity = 1.
	}
}

func (ds *DynamicSpreadBollWidthRatio) getAskSpread() (askSpread float64, err error) {
	askSpread, err = ds.AskSpreadScale.Scale(ds.getWeightedBBWidthRatio(true))
	if err != nil {
		log.WithError(err).Errorf("can not calculate dynamicAskSpread")
		return 0, err
	}

	return askSpread, nil
}

func (ds *DynamicSpreadBollWidthRatio) getBidSpread() (bidSpread float64, err error) {
	bidSpread, err = ds.BidSpreadScale.Scale(ds.getWeightedBBWidthRatio(false))
	if err != nil {
		log.WithError(err).Errorf("can not calculate dynamicAskSpread")
		return 0, err
	}

	return bidSpread, nil
}

func (ds *DynamicSpreadBollWidthRatio) getWeightedBBWidthRatio(positiveSigmoid bool) float64 {
	// Weight the width of Boll bands with sigmoid function and calculate the ratio after integral.
	//
	// Given the default band: moving average default_BB_mid, band from default_BB_lower to default_BB_upper.
	// And the neutral band: from neutral_BB_lower to neutral_BB_upper.
	// And a sensitivity factor alpha, which is a positive constant.
	//
	// width of default BB w = default_BB_upper - default_BB_lower
	//
	//                                         1                  x - default_BB_mid
	// sigmoid weighting function f(y) = ------------- where y = --------------------
	//                                    1 + exp(-y)                 w / alpha
	// Set the sigmoid weighting function:
	//   - To ask spread, the weighting density function d_weight(x) is sigmoid((x - default_BB_mid) / (w / alpha))
	//   - To bid spread, the weighting density function d_weight(x) is sigmoid((default_BB_mid - x) / (w / alpha))
	//   - The higher sensitivity factor alpha, the sharper weighting function.
	//
	// Then calculate the weighted bandwidth ratio by taking integral of d_weight(x) from neutral_BB_lower to neutral_BB_upper:
	//   infinite integral of ask spread sigmoid weighting density function F(x) = (w / alpha) * ln(exp(x / (w / alpha)) + exp(default_BB_mid / (w / alpha)))
	//   infinite integral of bid spread sigmoid weighting density function F(x) = x - (w / alpha) * ln(exp(x / (w / alpha)) + exp(default_BB_mid / (w / alpha)))
	//   Note that we've rescaled the sigmoid function to fit default BB,
	//   the weighted default BB width is always calculated by integral(f of x from default_BB_lower to default_BB_upper)
	//                     F(neutral_BB_upper) - F(neutral_BB_lower)
	//   weighted ratio = -------------------------------------------
	//                     F(default_BB_upper) - F(default_BB_lower)
	//   - The wider neutral band get greater ratio
	//   - To ask spread, the higher neutral band get greater ratio
	//   - To bid spread, the lower neutral band get greater ratio

	defaultMid := ds.defaultBoll.SMA.Last(0)
	defaultUpper := ds.defaultBoll.UpBand.Last(0)
	defaultLower := ds.defaultBoll.DownBand.Last(0)
	defaultWidth := defaultUpper - defaultLower
	neutralUpper := ds.neutralBoll.UpBand.Last(0)
	neutralLower := ds.neutralBoll.DownBand.Last(0)
	factor := defaultWidth / ds.Sensitivity
	var weightedUpper, weightedLower, weightedDivUpper, weightedDivLower float64
	if positiveSigmoid {
		weightedUpper = factor * math.Log(math.Exp(neutralUpper/factor)+math.Exp(defaultMid/factor))
		weightedLower = factor * math.Log(math.Exp(neutralLower/factor)+math.Exp(defaultMid/factor))
		weightedDivUpper = factor * math.Log(math.Exp(defaultUpper/factor)+math.Exp(defaultMid/factor))
		weightedDivLower = factor * math.Log(math.Exp(defaultLower/factor)+math.Exp(defaultMid/factor))
	} else {
		weightedUpper = neutralUpper - factor*math.Log(math.Exp(neutralUpper/factor)+math.Exp(defaultMid/factor))
		weightedLower = neutralLower - factor*math.Log(math.Exp(neutralLower/factor)+math.Exp(defaultMid/factor))
		weightedDivUpper = defaultUpper - factor*math.Log(math.Exp(defaultUpper/factor)+math.Exp(defaultMid/factor))
		weightedDivLower = defaultLower - factor*math.Log(math.Exp(defaultLower/factor)+math.Exp(defaultMid/factor))
	}
	return (weightedUpper - weightedLower) / (weightedDivUpper - weightedDivLower)
}
