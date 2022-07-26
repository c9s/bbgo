package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var (
	debugEWMA = false
	debugSMA  = false
	debugBOLL = false
)

func init() {
	// when using --dotenv option, the dotenv is loaded from command.PersistentPreRunE, not init.
	// hence here the env var won't enable the debug flag
	util.SetEnvVarBool("DEBUG_EWMA", &debugEWMA)
	util.SetEnvVarBool("DEBUG_SMA", &debugSMA)
	util.SetEnvVarBool("DEBUG_BOLL", &debugBOLL)
}

type StandardIndicatorSet struct {
	Symbol string
	// Standard indicators
	// interval -> window
	sma   map[types.IntervalWindow]*indicator.SMA
	ewma  map[types.IntervalWindow]*indicator.EWMA
	boll  map[types.IntervalWindowBandWidth]*indicator.BOLL
	stoch map[types.IntervalWindow]*indicator.STOCH

	stream types.Stream
	store  *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, stream types.Stream, store *MarketDataStore) *StandardIndicatorSet {
	return &StandardIndicatorSet{
		Symbol: symbol,
		sma:    make(map[types.IntervalWindow]*indicator.SMA),
		ewma:   make(map[types.IntervalWindow]*indicator.EWMA),
		boll:   make(map[types.IntervalWindowBandWidth]*indicator.BOLL),
		stoch:  make(map[types.IntervalWindow]*indicator.STOCH),
		store:  store,
		stream: stream,
	}
}

// BOLL returns the bollinger band indicator of the given interval, the window and bandwidth
func (set *StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandWidth float64) *indicator.BOLL {
	iwb := types.IntervalWindowBandWidth{IntervalWindow: iw, BandWidth: bandWidth}
	inc, ok := set.boll[iwb]
	if !ok {
		inc = &indicator.BOLL{IntervalWindow: iw, K: bandWidth}

		if klines, ok := set.store.KLinesOfInterval(iw.Interval); ok {
			for _, k := range *klines {
				inc.PushK(k)
			}
		}

		if debugBOLL {
			inc.OnUpdate(func(sma float64, upBand float64, downBand float64) {
				logrus.Infof("%s BOLL %s: sma=%f up=%f down=%f", set.Symbol, iw.String(), sma, upBand, downBand)
			})
		}

		inc.BindK(set.stream, set.Symbol, iw.Interval)
		set.boll[iwb] = inc
	}

	return inc
}

// SMA returns the simple moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	inc, ok := set.sma[iw]
	if !ok {
		inc = &indicator.SMA{IntervalWindow: iw}

		if klines, ok := set.store.KLinesOfInterval(iw.Interval); ok {
			for _, k := range *klines {
				inc.PushK(k)
			}
		}

		if debugSMA {
			inc.OnUpdate(func(value float64) {
				logrus.Infof("%s SMA %s: %f", set.Symbol, iw.String(), value)
			})
		}

		inc.BindK(set.stream, set.Symbol, iw.Interval)
		set.sma[iw] = inc
	}

	return inc
}

// EWMA returns the exponential weighed moving average indicator of the given interval and the window size.
func (set *StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA {
	inc, ok := set.ewma[iw]
	if !ok {
		inc = &indicator.EWMA{IntervalWindow: iw}

		if klines, ok := set.store.KLinesOfInterval(iw.Interval); ok {
			for _, k := range *klines {
				inc.PushK(k)
			}
		}

		if debugEWMA {
			inc.OnUpdate(func(value float64) {
				logrus.Infof("%s EWMA %s: value=%f", set.Symbol, iw.String(), value)
			})
		}

		inc.BindK(set.stream, set.Symbol, iw.Interval)
		set.ewma[iw] = inc
	}

	return inc
}

func (set *StandardIndicatorSet) STOCH(iw types.IntervalWindow) *indicator.STOCH {
	inc, ok := set.stoch[iw]
	if !ok {
		inc = &indicator.STOCH{IntervalWindow: iw}

		if klines, ok := set.store.KLinesOfInterval(iw.Interval); ok {
			for _, k := range *klines {
				inc.PushK(k)
			}
		}

		inc.BindK(set.stream, set.Symbol, iw.Interval)
		set.stoch[iw] = inc
	}

	return inc
}
