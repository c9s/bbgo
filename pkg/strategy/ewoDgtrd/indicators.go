package ewoDgtrd

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// The indicators of this strategy are fed manually from the MarketDataStore kline window callbacks
// (which also lazily back-fill the indicators from the whole window), so they are built on manually fed sources.

// pushFloat64 pushes the value into a manually fed source series, emits it to the subscribers,
// and keeps the source series bounded.
func pushFloat64(s *types.Float64Series, v float64) {
	s.PushAndEmit(v)
	s.Slice = types.ShrinkSlice(s.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
}

// klineFeed is a KLineSubscription that is fed manually by the strategy.
// Unlike indicatorv2.KLineStream, it only keeps the last kline, so it does not grow unbounded.
type klineFeed struct {
	subscribers []func(k types.KLine)
	last        *types.KLine
	length      int
}

func (f *klineFeed) AddSubscriber(fn func(k types.KLine)) {
	f.subscribers = append(f.subscribers, fn)
}

func (f *klineFeed) Length() int {
	return f.length
}

func (f *klineFeed) Last(i int) *types.KLine {
	if i != 0 {
		return nil
	}
	return f.last
}

func (f *klineFeed) Push(k types.KLine) {
	f.last = &k
	f.length++
	for _, fn := range f.subscribers {
		fn(k)
	}
}

// ewma is a v2 EWMA stream fed manually through Update
type ewma struct {
	*indicatorv2.EWMAStream
	input *types.Float64Series
}

func newEWMA(window int) *ewma {
	input := types.NewFloat64Series()
	return &ewma{EWMAStream: indicatorv2.EWMA2(input, window), input: input}
}

func (s *ewma) Update(v float64) {
	pushFloat64(s.input, v)
}

// sma is a v2 SMA stream fed manually through Update.
//
// The v2 SMA emits partial window means from the first value, while the legacy SMA had no value until its window
// was filled. sma only exposes the values computed with a filled window (Length is 0 and Last returns 0 before that),
// like the legacy SMA did.
//
// sma does not embed the stream on purpose: the SeriesExtend helpers of the embedded stream would read
// the partial window values. Wrap it with types.NewSeries to get a SeriesExtend.
type sma struct {
	stream *indicatorv2.SMAStream
	input  *types.Float64Series
	window int
	count  int
}

func newSMA(window int) *sma {
	input := types.NewFloat64Series()
	return &sma{stream: indicatorv2.SMA(input, window), input: input, window: window}
}

func (s *sma) Update(v float64) {
	s.count++
	pushFloat64(s.input, v)
}

func (s *sma) Length() int {
	return max(0, min(s.stream.Length(), s.count-s.window+1))
}

func (s *sma) Last(i int) float64 {
	if i < 0 || i >= s.Length() {
		return 0
	}
	return s.stream.Last(i)
}

func (s *sma) Index(i int) float64 {
	return s.Last(i)
}

var _ types.Series = &sma{}

// CCISTOCH is the stochastic oscillator of the CCI
//
// Refer: https://tw.tradingview.com/script/XZyG5SOx-CCI-Stochastic-and-a-quick-lesson-on-Scalping-Trading-Systems/
type CCISTOCH struct {
	input *types.Float64Series
	cci   *indicatorv2.CCIStream
	// stochFeed feeds stoch with klines whose high, low and close are the last CCI value
	stochFeed  *klineFeed
	stoch      *indicatorv2.StochStream
	ma         *sma
	filterHigh float64
	filterLow  float64
}

func NewCCISTOCH(_ types.Interval, filterHigh, filterLow float64) *CCISTOCH {
	input := types.NewFloat64Series()
	stochFeed := &klineFeed{}
	return &CCISTOCH{
		input:      input,
		cci:        indicatorv2.CCI(input, 28),
		stochFeed:  stochFeed,
		stoch:      indicatorv2.Stoch(stochFeed, 28, indicatorv2.DPeriod),
		ma:         newSMA(3),
		filterHigh: filterHigh,
		filterLow:  filterLow,
	}
}

func (inc *CCISTOCH) Update(cloze float64) {
	pushFloat64(inc.input, cloze)
	// like the legacy implementation, the stochastic oscillator is updated on every value,
	// with 0 while the CCI window is not filled yet.
	c := fixedpoint.NewFromFloat(inc.cci.Last(0))
	inc.stochFeed.Push(types.KLine{High: c, Low: c, Close: c})
	inc.ma.Update(inc.stoch.D.Last(0))
}

func (inc *CCISTOCH) BuySignal() bool {
	hasGrey := false
	for i := 0; i < inc.ma.Length(); i++ {
		v := inc.ma.Index(i)
		if v > inc.filterHigh {
			return false
		} else if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		} else if v < inc.filterLow {
			return hasGrey
		}
	}
	return false
}

func (inc *CCISTOCH) SellSignal() bool {
	hasGrey := false
	for i := 0; i < inc.ma.Length(); i++ {
		v := inc.ma.Index(i)
		if v < inc.filterLow {
			return false
		} else if v >= inc.filterLow && v <= inc.filterHigh {
			hasGrey = true
			continue
		} else if v > inc.filterHigh {
			return hasGrey
		}
	}
	return false
}

// VWEMA is the volume weighted EMA: EWMA(price * volume) / EWMA(volume)
type VWEMA struct {
	PV *ewma
	V  *ewma
}

func NewVWEMA(window int) *VWEMA {
	return &VWEMA{PV: newEWMA(window), V: newEWMA(window)}
}

func (inc *VWEMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *VWEMA) Last(i int) float64 {
	if i >= inc.PV.Length() {
		return 0
	}
	vi := inc.V.Last(i)
	if vi == 0 {
		return 0
	}
	return inc.PV.Last(i) / vi
}

func (inc *VWEMA) Length() int {
	pvl := inc.PV.Length()
	vl := inc.V.Length()
	if pvl < vl {
		return pvl
	}
	return vl
}

func (inc *VWEMA) Update(kline types.KLine) {
	inc.PV.Update(kline.Close.Mul(kline.Volume).Float64())
	inc.V.Update(kline.Volume.Float64())
}

func (inc *VWEMA) UpdateVal(price float64, vol float64) {
	inc.PV.Update(price * vol)
	inc.V.Update(vol)
}
