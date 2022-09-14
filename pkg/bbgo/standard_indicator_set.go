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

type MACDConfig struct {
	types.IntervalWindow
}

type StandardIndicatorSet struct {
	Symbol string

	// Standard indicators
	// interval -> window
	iwbIndicators  map[types.IntervalWindowBandWidth]*indicator.BOLL
	iwIndicators   map[indicatorKey]indicator.KLinePusher
	macdIndicators map[indicator.MACDConfig]*indicator.MACD

	stream types.Stream
	store  *MarketDataStore
}

type indicatorKey struct {
	iw types.IntervalWindow
	id string
}

func NewStandardIndicatorSet(symbol string, stream types.Stream, store *MarketDataStore) *StandardIndicatorSet {
	return &StandardIndicatorSet{
		Symbol:         symbol,
		store:          store,
		stream:         stream,
		iwIndicators:   make(map[indicatorKey]indicator.KLinePusher),
		iwbIndicators:  make(map[types.IntervalWindowBandWidth]*indicator.BOLL),
		macdIndicators: make(map[indicator.MACDConfig]*indicator.MACD),
	}
}

func (s *StandardIndicatorSet) initAndBind(inc indicator.KLinePusher, interval types.Interval) {
	if klines, ok := s.store.KLinesOfInterval(interval); ok {
		for _, k := range *klines {
			inc.PushK(k)
		}
	}

	s.stream.OnKLineClosed(types.KLineWith(s.Symbol, interval, inc.PushK))
}

func (s *StandardIndicatorSet) allocateSimpleIndicator(t indicator.KLinePusher, iw types.IntervalWindow, id string) indicator.KLinePusher {
	k := indicatorKey{
		iw: iw,
		id: id,
	}
	inc, ok := s.iwIndicators[k]
	if ok {
		return inc
	}

	inc = t
	s.initAndBind(inc, iw.Interval)
	s.iwIndicators[k] = inc
	return t
}

// SMA is a helper function that returns the simple moving average indicator of the given interval and the window size.
func (s *StandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	inc := s.allocateSimpleIndicator(&indicator.SMA{IntervalWindow: iw}, iw, "sma")
	return inc.(*indicator.SMA)
}

// EWMA is a helper function that returns the exponential weighed moving average indicator of the given interval and the window size.
func (s *StandardIndicatorSet) EWMA(iw types.IntervalWindow) *indicator.EWMA {
	inc := s.allocateSimpleIndicator(&indicator.EWMA{IntervalWindow: iw}, iw, "ewma")
	return inc.(*indicator.EWMA)
}

// VWMA
func (s *StandardIndicatorSet) VWMA(iw types.IntervalWindow) *indicator.VWMA {
	inc := s.allocateSimpleIndicator(&indicator.VWMA{IntervalWindow: iw}, iw, "vwma")
	return inc.(*indicator.VWMA)
}

func (s *StandardIndicatorSet) PivotHigh(iw types.IntervalWindow) *indicator.PivotHigh {
	inc := s.allocateSimpleIndicator(&indicator.PivotHigh{IntervalWindow: iw}, iw, "pivothigh")
	return inc.(*indicator.PivotHigh)
}

func (s *StandardIndicatorSet) PivotLow(iw types.IntervalWindow) *indicator.PivotLow {
	inc := s.allocateSimpleIndicator(&indicator.PivotLow{IntervalWindow: iw}, iw, "pivotlow")
	return inc.(*indicator.PivotLow)
}

func (s *StandardIndicatorSet) ATR(iw types.IntervalWindow) *indicator.ATR {
	inc := s.allocateSimpleIndicator(&indicator.ATR{IntervalWindow: iw}, iw, "atr")
	return inc.(*indicator.ATR)
}

func (s *StandardIndicatorSet) ATRP(iw types.IntervalWindow) *indicator.ATRP {
	inc := s.allocateSimpleIndicator(&indicator.ATRP{IntervalWindow: iw}, iw, "atrp")
	return inc.(*indicator.ATRP)
}

func (s *StandardIndicatorSet) EMV(iw types.IntervalWindow) *indicator.EMV {
	inc := s.allocateSimpleIndicator(&indicator.EMV{IntervalWindow: iw}, iw, "emv")
	return inc.(*indicator.EMV)
}

func (s *StandardIndicatorSet) CCI(iw types.IntervalWindow) *indicator.CCI {
	inc := s.allocateSimpleIndicator(&indicator.CCI{IntervalWindow: iw}, iw, "cci")
	return inc.(*indicator.CCI)
}

func (s *StandardIndicatorSet) HULL(iw types.IntervalWindow) *indicator.HULL {
	inc := s.allocateSimpleIndicator(&indicator.HULL{IntervalWindow: iw}, iw, "hull")
	return inc.(*indicator.HULL)
}

func (s *StandardIndicatorSet) STOCH(iw types.IntervalWindow) *indicator.STOCH {
	inc := s.allocateSimpleIndicator(&indicator.STOCH{IntervalWindow: iw}, iw, "stoch")
	return inc.(*indicator.STOCH)
}

// BOLL returns the bollinger band indicator of the given interval, the window and bandwidth
func (s *StandardIndicatorSet) BOLL(iw types.IntervalWindow, bandWidth float64) *indicator.BOLL {
	iwb := types.IntervalWindowBandWidth{IntervalWindow: iw, BandWidth: bandWidth}
	inc, ok := s.iwbIndicators[iwb]
	if !ok {
		inc = &indicator.BOLL{IntervalWindow: iw, K: bandWidth}
		s.initAndBind(inc, iw.Interval)

		if debugBOLL {
			inc.OnUpdate(func(sma float64, upBand float64, downBand float64) {
				logrus.Infof("%s BOLL %s: sma=%f up=%f down=%f", s.Symbol, iw.String(), sma, upBand, downBand)
			})
		}
		s.iwbIndicators[iwb] = inc
	}

	return inc
}

func (s *StandardIndicatorSet) MACD(iw types.IntervalWindow, shortPeriod, longPeriod int) *indicator.MACD {
	config := indicator.MACDConfig{IntervalWindow: iw, ShortPeriod: shortPeriod, LongPeriod: longPeriod}

	inc, ok := s.macdIndicators[config]
	if ok {
		return inc
	}
	inc = &indicator.MACD{MACDConfig: config}
	s.macdIndicators[config] = inc
	s.initAndBind(inc, config.IntervalWindow.Interval)
	return inc
}

// GHFilter is a helper function that returns the G-H (alpha beta) digital filter of the given interval and the window size.
func (s *StandardIndicatorSet) GHFilter(iw types.IntervalWindow) *indicator.GHFilter {
	inc := s.allocateSimpleIndicator(&indicator.GHFilter{IntervalWindow: iw}, iw, "ghfilter")
	return inc.(*indicator.GHFilter)
}

// KalmanFilter is a helper function that returns the Kalman digital filter of the given interval and the window size.
// Note that the additional smooth window is set to zero in standard indicator set. Users have to create their own instance and push K-lines if a smoother filter is needed.
func (s *StandardIndicatorSet) KalmanFilter(iw types.IntervalWindow) *indicator.KalmanFilter {
	inc := s.allocateSimpleIndicator(&indicator.KalmanFilter{IntervalWindow: iw, AdditionalSmoothWindow: 0}, iw, "kalmanfilter")
	return inc.(*indicator.KalmanFilter)
}
