package indicatorv2

import (
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultEMVScale is the default volume scale of the EMV box ratio
const DefaultEMVScale float64 = 100000000.

// EMVStream is the Ease of Movement indicator.
//
// Refer: Ease of Movement
// Refer URL: https://www.investopedia.com/terms/e/easeofmovement.asp
//
// The Ease of Movement (EOM) is a technical analysis indicator that is used to measure the relationship between the volume of a security
// and its price movement. It is calculated by dividing the difference between the high and low prices of the security by the total
// volume of the security over a specified period of time, and then multiplying the result by a constant factor. This resulting value is
// then plotted on the price chart as a line, which can be used to make predictions about future price movements. The EOM is typically
// used to identify periods of high or low trading activity, and can be used to confirm other technical analysis signals.
//
// Calculation:
//
//	distanceMoved = (high + low) / 2 - (prevHigh + prevLow) / 2
//	boxRatio      = (volume / scale) / (high - low)
//	raw EMV       = distanceMoved / boxRatio
//	EMV           = SMA(raw EMV, window)
//
// The first kline only initializes the previous high/low. The raw (1-period) EMV values
// are exposed in the Raw series, and the smoothed EMV is emitted once window raw values
// have been collected.
type EMVStream struct {
	// the smoothed EMV series
	*types.Float64Series

	// Raw is the 1-period EMV series
	Raw *types.Float64Series

	// SMA is the moving average of the raw EMV series (including the warm-up values)
	SMA *SMAStream

	window       int
	scale        float64
	prevH, prevL float64
	initialized  bool
	count        int
}

// EMV creates the Ease of Movement stream. If scale is zero, DefaultEMVScale is used.
func EMV(source KLineSubscription, window int, scale float64) *EMVStream {
	checkWindow(window)

	if scale == 0 {
		scale = DefaultEMVScale
	}

	raw := types.NewFloat64Series()
	s := &EMVStream{
		Float64Series: types.NewFloat64Series(),
		Raw:           raw,
		SMA:           SMA(raw, window),
		window:        window,
		scale:         scale,
	}

	source.AddSubscriber(func(k types.KLine) {
		s.Update(k.High.Float64(), k.Low.Float64(), k.Volume.Float64())
	})
	return s
}

// Update pushes the high, low and volume values into the stream.
func (s *EMVStream) Update(high, low, vol float64) {
	if !s.initialized {
		s.initialized = true
		s.prevH = high
		s.prevL = low
		return
	}

	distanceMoved := (high+low)/2. - (s.prevH+s.prevL)/2.
	boxRatio := vol / s.scale / (high - low)
	result := distanceMoved / boxRatio
	s.prevH = high
	s.prevL = low

	s.Raw.PushAndEmit(result)
	s.count++

	if s.count >= s.window {
		s.PushAndEmit(s.SMA.Last(0))
	}

	s.Truncate()
}

func (s *EMVStream) Truncate() {
	s.Slice = generalTruncate(s.Slice)
	s.Raw.Slice = generalTruncate(s.Raw.Slice)
}
