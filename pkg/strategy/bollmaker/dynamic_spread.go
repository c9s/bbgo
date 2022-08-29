package bollmaker

import (
	"github.com/pkg/errors"
	"math"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

type DynamicSpreadSettings struct {
	AmpSpreadSettings                    *DynamicSpreadAmpSettings            `json:"amplitude"`
	WeightedBollWidthRatioSpreadSettings *DynamicSpreadBollWidthRatioSettings `json:"weightedBollWidth"`

	// deprecated
	Enabled *bool `json:"enabled"`

	// deprecated
	types.IntervalWindow

	// deprecated. AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// deprecated. BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`
}

// Initialize dynamic spreads and preload SMAs
func (ds *DynamicSpreadSettings) Initialize(symbol string, session *bbgo.ExchangeSession, neutralBoll, defaultBoll *indicator.BOLL) {
	switch {
	case ds.Enabled != nil && !*ds.Enabled:
		// do nothing
	case ds.AmpSpreadSettings != nil:
		ds.AmpSpreadSettings.initialize(symbol, session)
	case ds.WeightedBollWidthRatioSpreadSettings != nil:
		ds.WeightedBollWidthRatioSpreadSettings.initialize(neutralBoll, defaultBoll)
	}
}

func (ds *DynamicSpreadSettings) IsEnabled() bool {
	return ds.AmpSpreadSettings != nil || ds.WeightedBollWidthRatioSpreadSettings != nil
}

// Update dynamic spreads
func (ds *DynamicSpreadSettings) Update(kline types.KLine) {
	switch {
	case ds.AmpSpreadSettings != nil:
		ds.AmpSpreadSettings.update(kline)
	case ds.WeightedBollWidthRatioSpreadSettings != nil:
		// Boll bands are updated outside of settings. Do nothing.
	default:
		// Disabled. Do nothing.
	}
}

// GetAskSpread returns current ask spread
func (ds *DynamicSpreadSettings) GetAskSpread() (askSpread float64, err error) {
	switch {
	case ds.AmpSpreadSettings != nil:
		return ds.AmpSpreadSettings.getAskSpread()
	case ds.WeightedBollWidthRatioSpreadSettings != nil:
		return ds.WeightedBollWidthRatioSpreadSettings.getAskSpread()
	default:
		return 0, errors.New("dynamic spread is not enabled")
	}
}

// GetBidSpread returns current dynamic bid spread
func (ds *DynamicSpreadSettings) GetBidSpread() (bidSpread float64, err error) {
	switch {
	case ds.AmpSpreadSettings != nil:
		return ds.AmpSpreadSettings.getBidSpread()
	case ds.WeightedBollWidthRatioSpreadSettings != nil:
		return ds.WeightedBollWidthRatioSpreadSettings.getBidSpread()
	default:
		return 0, errors.New("dynamic spread is not enabled")
	}
}

type DynamicSpreadAmpSettings struct {
	types.IntervalWindow

	// AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`

	dynamicAskSpread *indicator.SMA
	dynamicBidSpread *indicator.SMA
}

func (ds *DynamicSpreadAmpSettings) initialize(symbol string, session *bbgo.ExchangeSession) {
	ds.dynamicBidSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}
	ds.dynamicAskSpread = &indicator.SMA{IntervalWindow: types.IntervalWindow{Interval: ds.Interval, Window: ds.Window}}
	kLineStore, _ := session.MarketDataStore(symbol)
	if klines, ok := kLineStore.KLinesOfInterval(ds.Interval); ok {
		for i := 0; i < len(*klines); i++ {
			ds.update((*klines)[i])
		}
	}
}

func (ds *DynamicSpreadAmpSettings) update(kline types.KLine) {
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

func (ds *DynamicSpreadAmpSettings) getAskSpread() (askSpread float64, err error) {
	if ds.AskSpreadScale != nil && ds.dynamicAskSpread.Length() >= ds.Window {
		askSpread, err = ds.AskSpreadScale.Scale(ds.dynamicAskSpread.Last())
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicAskSpread")
			return 0, err
		}

		return askSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}

func (ds *DynamicSpreadAmpSettings) getBidSpread() (bidSpread float64, err error) {
	if ds.BidSpreadScale != nil && ds.dynamicBidSpread.Length() >= ds.Window {
		bidSpread, err = ds.BidSpreadScale.Scale(ds.dynamicBidSpread.Last())
		if err != nil {
			log.WithError(err).Errorf("can not calculate dynamicBidSpread")
			return 0, err
		}

		return bidSpread, nil
	}

	return 0, errors.New("incomplete dynamic spread settings or not enough data yet")
}

type DynamicSpreadBollWidthRatioSettings struct {
	// AskSpreadScale is used to define the ask spread range with the given percentage.
	AskSpreadScale *bbgo.PercentageScale `json:"askSpreadScale"`

	// BidSpreadScale is used to define the bid spread range with the given percentage.
	BidSpreadScale *bbgo.PercentageScale `json:"bidSpreadScale"`

	neutralBoll *indicator.BOLL
	defaultBoll *indicator.BOLL
}

func (ds *DynamicSpreadBollWidthRatioSettings) initialize(neutralBoll, defaultBoll *indicator.BOLL) {
	ds.neutralBoll = neutralBoll
	ds.defaultBoll = defaultBoll
}

func (ds *DynamicSpreadBollWidthRatioSettings) getAskSpread() (askSpread float64, err error) {
	askSpread, err = ds.AskSpreadScale.Scale(ds.getWeightedBBWidthRatio(true))
	if err != nil {
		log.WithError(err).Errorf("can not calculate dynamicAskSpread")
		return 0, err
	}

	return askSpread, nil
}

func (ds *DynamicSpreadBollWidthRatioSettings) getBidSpread() (bidSpread float64, err error) {
	bidSpread, err = ds.BidSpreadScale.Scale(ds.getWeightedBBWidthRatio(false))
	if err != nil {
		log.WithError(err).Errorf("can not calculate dynamicAskSpread")
		return 0, err
	}

	return bidSpread, nil
}

func (ds *DynamicSpreadBollWidthRatioSettings) getWeightedBBWidthRatio(positiveSigmoid bool) float64 {
	// Weight the width of Boll bands with sigmoid function and calculate the ratio after integral.
	//
	// Given the default band: moving average default_BB_mid, band from default_BB_lower to default_BB_upper.
	// And the neutral band: from neutral_BB_lower to neutral_BB_upper.
	//
	//                                         1                  x - default_BB_mid
	// sigmoid weighting function f(y) = ------------- where y = --------------------
	//                                    1 + exp(-y)              default_BB_width
	// Set the sigmoid weighting function:
	//   - to ask spread, the weighting density function d_weight(x) is sigmoid((x - default_BB_mid) / (default_BB_upper - default_BB_lower))
	//   - to bid spread, the weighting density function d_weight(x) is sigmoid((default_BB_mid - x) / (default_BB_upper - default_BB_lower))
	//
	// Then calculate the weighted band width ratio by taking integral of d_weight(x) from bx_lower to bx_upper:
	//   infinite integral of ask spread sigmoid weighting density function F(y) = ln(1 + exp(y))
	//   infinite integral of bid spread sigmoid weighting density function F(y) = y - ln(1 + exp(y))
	//   Note that we've rescaled the sigmoid function to fit default BB,
	//   the weighted default BB width is always calculated by integral(f of y from -1 to 1) = F(1) - F(-1)
	//                     F(y_upper) - F(y_lower)     F(y_upper) - F(y_lower)
	//   weighted ratio = ------------------------- = -------------------------
	//                          F(1) - F(-1)                      1
	//     where y_upper = (neutral_BB_upper - default_BB_mid) / default_BB_width
	//           y_lower = (neutral_BB_lower - default_BB_mid) / default_BB_width
	//   - The wider neutral band get greater ratio
	//   - To ask spread, the higher neutral band get greater ratio
	//   - To bid spread, the lower neutral band get greater ratio

	defaultMid := ds.defaultBoll.SMA.Last()
	defaultWidth := ds.defaultBoll.UpBand.Last() - ds.defaultBoll.DownBand.Last()
	yUpper := (ds.neutralBoll.UpBand.Last() - defaultMid) / defaultWidth
	yLower := (ds.neutralBoll.DownBand.Last() - defaultMid) / defaultWidth
	var weightedUpper, weightedLower float64
	if positiveSigmoid {
		weightedUpper = math.Log(1 + math.Pow(math.E, yUpper))
		weightedLower = math.Log(1 + math.Pow(math.E, yLower))
	} else {
		weightedUpper = yUpper - math.Log(1+math.Pow(math.E, yUpper))
		weightedLower = yLower - math.Log(1+math.Pow(math.E, yLower))
	}
	// The weighted ratio always positive, and may be greater than 1 if neutral band is wider than default band.
	return (weightedUpper - weightedLower) / 1.
}
