package bbgo

import (
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var (
	debugBOLL = false
)

func init() {
	// when using --dotenv option, the dotenv is loaded from command.PersistentPreRunE, not init.
	// hence here the env var won't enable the debug flag
	util.SetEnvVarBool("DEBUG_BOLL", &debugBOLL)
}

type StandardIndicatorSet struct {
	Symbol string

	// Standard indicators
	// interval -> window
	boll    map[types.IntervalWindowBandWidth]*indicator.BOLL
	stoch   map[types.IntervalWindow]*indicator.STOCH
	simples map[types.IntervalWindow]indicator.Simple

	stream types.Stream
	store  *MarketDataStore
}

func NewStandardIndicatorSet(symbol string, stream types.Stream, store *MarketDataStore) *StandardIndicatorSet {
	return &StandardIndicatorSet{
		Symbol:  symbol,
		store:   store,
		stream:  stream,
		simples: make(map[types.IntervalWindow]indicator.Simple),

		boll:  make(map[types.IntervalWindowBandWidth]*indicator.BOLL),
		stoch: make(map[types.IntervalWindow]*indicator.STOCH),
	}
}

func (s *StandardIndicatorSet) initAndBind(inc indicator.KLinePusher, iw types.IntervalWindow) {
	if klines, ok := s.store.KLinesOfInterval(iw.Interval); ok {
		for _, k := range *klines {
			inc.PushK(k)
		}
	}

	s.stream.OnKLineClosed(types.KLineWith(s.Symbol, iw.Interval, inc.PushK))
}

func (s *StandardIndicatorSet) allocateSimpleIndicator(t indicator.Simple, iw types.IntervalWindow) indicator.Simple {
	inc, ok := s.simples[iw]
	if ok {
		return inc
	}

	inc = t
	s.initAndBind(inc, iw)
	s.simples[iw] = inc
	return t
}

// SMA is a helper function that returns the simple moving average indicator of the given interval and the window size.
func (s *StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	inc := s.allocateSimpleIndicator(&indicator.SMA{IntervalWindow: iw}, iw)
	return inc.(*indicator.SMA)
}

// EWMA is a helper function that returns the exponential weighed moving average indicator of the given interval and the window size.
func (s *StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA {
	inc := s.allocateSimpleIndicator(&indicator.EWMA{IntervalWindow: iw}, iw)
	return inc.(*indicator.EWMA)
}

func (s *StandardIndicatorSet) PivotLow(iw types.IntervalWindow) *indicator.PivotLow {
	inc := s.allocateSimpleIndicator(&indicator.PivotLow{IntervalWindow: iw}, iw)
	return inc.(*indicator.PivotLow)
}

func (s *StandardIndicatorSet) STOCH(iw types.IntervalWindow) *indicator.STOCH {
	inc, ok := s.stoch[iw]
	if !ok {
		inc = &indicator.STOCH{IntervalWindow: iw}
		s.initAndBind(inc, iw)
		s.stoch[iw] = inc
	}

	return inc
}

// BOLL returns the bollinger band indicator of the given interval, the window and bandwidth
func (s *StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandWidth float64) *indicator.BOLL {
	iwb := types.IntervalWindowBandWidth{IntervalWindow: iw, BandWidth: bandWidth}
	inc, ok := s.boll[iwb]
	if !ok {
		inc = &indicator.BOLL{IntervalWindow: iw, K: bandWidth}
		s.initAndBind(inc, iw)

		if debugBOLL {
			inc.OnUpdate(func(sma float64, upBand float64, downBand float64) {
				logrus.Infof("%s BOLL %s: sma=%f up=%f down=%f", s.Symbol, iw.String(), sma, upBand, downBand)
			})
		}
		s.boll[iwb] = inc
	}

	return inc
}
