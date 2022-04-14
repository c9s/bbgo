package types

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util"
)

type Direction int

const DirectionUp = 1
const DirectionNone = 0
const DirectionDown = -1

var Two = fixedpoint.NewFromInt(2)

type KLineOrWindow interface {
	GetInterval() string
	Direction() Direction
	GetChange() fixedpoint.Value
	GetMaxChange() fixedpoint.Value
	GetThickness() fixedpoint.Value

	Mid() fixedpoint.Value
	GetOpen() fixedpoint.Value
	GetClose() fixedpoint.Value
	GetHigh() fixedpoint.Value
	GetLow() fixedpoint.Value

	BounceUp() bool
	BounceDown() bool
	GetUpperShadowRatio() fixedpoint.Value
	GetLowerShadowRatio() fixedpoint.Value

	SlackAttachment() slack.Attachment
}

type KLineQueryOptions struct {
	Limit     int
	StartTime *time.Time
	EndTime   *time.Time
}

// KLine uses binance's kline as the standard structure
type KLine struct {
	GID      uint64       `json:"gid" db:"gid"`
	Exchange ExchangeName `json:"exchange" db:"exchange"`

	Symbol string `json:"symbol" db:"symbol"`

	StartTime Time `json:"startTime" db:"start_time"`
	EndTime   Time `json:"endTime" db:"end_time"`

	Interval Interval `json:"interval" db:"interval"`

	Open                     fixedpoint.Value `json:"open" db:"open"`
	Close                    fixedpoint.Value `json:"close" db:"close"`
	High                     fixedpoint.Value `json:"high" db:"high"`
	Low                      fixedpoint.Value `json:"low" db:"low"`
	Volume                   fixedpoint.Value `json:"volume" db:"volume"`
	QuoteVolume              fixedpoint.Value `json:"quoteVolume" db:"quote_volume"`
	TakerBuyBaseAssetVolume  fixedpoint.Value `json:"takerBuyBaseAssetVolume" db:"taker_buy_base_volume"`
	TakerBuyQuoteAssetVolume fixedpoint.Value `json:"takerBuyQuoteAssetVolume" db:"taker_buy_quote_volume"`

	LastTradeID    uint64 `json:"lastTradeID" db:"last_trade_id"`
	NumberOfTrades uint64 `json:"numberOfTrades" db:"num_trades"`
	Closed         bool   `json:"closed" db:"closed"`
}

func (k KLine) GetStartTime() Time {
	return k.StartTime
}

func (k KLine) GetEndTime() Time {
	return k.EndTime
}

func (k KLine) GetInterval() Interval {
	return k.Interval
}

func (k KLine) Mid() fixedpoint.Value {
	return k.High.Add(k.Low).Div(Two)
}

// green candle with open and close near high price
func (k KLine) BounceUp() bool {
	mid := k.Mid()
	trend := k.Direction()
	return trend > 0 && k.Open.Compare(mid) > 0 && k.Close.Compare(mid) > 0
}

// red candle with open and close near low price
func (k KLine) BounceDown() bool {
	mid := k.Mid()
	trend := k.Direction()
	return trend > 0 && k.Open.Compare(mid) < 0 && k.Close.Compare(mid) < 0
}

func (k KLine) Direction() Direction {
	o := k.GetOpen()
	c := k.GetClose()

	if c.Compare(o) > 0 {
		return DirectionUp
	} else if c.Compare(o) < 0 {
		return DirectionDown
	}
	return DirectionNone
}

func (k KLine) GetHigh() fixedpoint.Value {
	return k.High
}

func (k KLine) GetLow() fixedpoint.Value {
	return k.Low
}

func (k KLine) GetOpen() fixedpoint.Value {
	return k.Open
}

func (k KLine) GetClose() fixedpoint.Value {
	return k.Close
}

func (k KLine) GetMaxChange() fixedpoint.Value {
	return k.GetHigh().Sub(k.GetLow())
}

func (k KLine) GetAmplification() fixedpoint.Value {
	return k.GetMaxChange().Div(k.GetLow())
}

// GetThickness returns the thickness of the kline. 1 => thick, 0.1 => thin
func (k KLine) GetThickness() fixedpoint.Value {
	out := k.GetChange().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLine) GetUpperShadowRatio() fixedpoint.Value {
	out := k.GetUpperShadowHeight().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLine) GetUpperShadowHeight() fixedpoint.Value {
	high := k.GetHigh()
	open := k.GetOpen()
	clos := k.GetClose()
	if open.Compare(clos) > 0 {
		return high.Sub(open)
	}
	return high.Sub(clos)
}

func (k KLine) GetLowerShadowRatio() fixedpoint.Value {
	out := k.GetLowerShadowHeight().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLine) GetLowerShadowHeight() fixedpoint.Value {
	low := k.Low
	if k.Open.Compare(k.Close) < 0 { // uptrend
		return k.Open.Sub(low)
	}

	// downtrend
	return k.Close.Sub(low)
}

// GetBody returns the height of the candle real body
func (k KLine) GetBody() fixedpoint.Value {
	return k.GetChange()
}

// GetChange returns Close price - Open price.
func (k KLine) GetChange() fixedpoint.Value {
	return k.Close.Sub(k.Open)
}

func (k KLine) Color() string {
	if k.Direction() > 0 {
		return GreenColor
	} else if k.Direction() < 0 {
		return RedColor
	}
	return GrayColor
}

func (k KLine) String() string {
	return fmt.Sprintf("%s %s %s %s O: %.4f H: %.4f L: %.4f C: %.4f CHG: %.4f MAXCHG: %.4f V: %.4f QV: %.2f TBBV: %.2f",
		k.Exchange.String(),
		k.StartTime.Time().Format("2006-01-02 15:04"),
		k.Symbol, k.Interval, k.Open.Float64(), k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.GetChange().Float64(), k.GetMaxChange().Float64(), k.Volume.Float64(), k.QuoteVolume.Float64(), k.TakerBuyBaseAssetVolume.Float64())
}

func (k KLine) PlainText() string {
	return k.String()
}

func (k KLine) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Text:  fmt.Sprintf("*%s* KLine %s", k.Symbol, k.Interval),
		Color: k.Color(),
		Fields: []slack.AttachmentField{
			{Title: "Open", Value: util.FormatValue(k.Open, 2), Short: true},
			{Title: "High", Value: util.FormatValue(k.High, 2), Short: true},
			{Title: "Low", Value: util.FormatValue(k.Low, 2), Short: true},
			{Title: "Close", Value: util.FormatValue(k.Close, 2), Short: true},
			{Title: "Mid", Value: util.FormatValue(k.Mid(), 2), Short: true},
			{Title: "Change", Value: util.FormatValue(k.GetChange(), 2), Short: true},
			{Title: "Volume", Value: util.FormatValue(k.Volume, 2), Short: true},
			{Title: "Taker Buy Base Volume", Value: util.FormatValue(k.TakerBuyBaseAssetVolume, 2), Short: true},
			{Title: "Taker Buy Quote Volume", Value: util.FormatValue(k.TakerBuyQuoteAssetVolume, 2), Short: true},
			{Title: "Max Change", Value: util.FormatValue(k.GetMaxChange(), 2), Short: true},
			{
				Title: "Thickness",
				Value: util.FormatValue(k.GetThickness(), 4),
				Short: true,
			},
			{
				Title: "UpperShadowRatio",
				Value: util.FormatValue(k.GetUpperShadowRatio(), 4),
				Short: true,
			},
			{
				Title: "LowerShadowRatio",
				Value: util.FormatValue(k.GetLowerShadowRatio(), 4),
				Short: true,
			},
		},
		Footer:     "",
		FooterIcon: "",
	}
}

type KLineWindow []KLine

// ReduceClose reduces the closed prices
func (k KLineWindow) ReduceClose() fixedpoint.Value {
	s := fixedpoint.Zero

	for _, kline := range k {
		s = s.Add(kline.GetClose())
	}

	return s
}

func (k KLineWindow) Len() int {
	return len(k)
}

func (k KLineWindow) First() KLine {
	return k[0]
}

func (k KLineWindow) Last() KLine {
	return k[len(k)-1]
}

func (k KLineWindow) GetInterval() Interval {
	return k.First().Interval
}

func (k KLineWindow) GetOpen() fixedpoint.Value {
	return k.First().GetOpen()
}

func (k KLineWindow) GetClose() fixedpoint.Value {
	end := len(k) - 1
	return k[end].GetClose()
}

func (k KLineWindow) GetHigh() fixedpoint.Value {
	high := k.First().GetHigh()
	for _, line := range k {
		high = fixedpoint.Max(high, line.GetHigh())
	}

	return high
}

func (k KLineWindow) GetLow() fixedpoint.Value {
	low := k.First().GetLow()
	for _, line := range k {
		low = fixedpoint.Min(low, line.GetLow())
	}

	return low
}

func (k KLineWindow) GetChange() fixedpoint.Value {
	return k.GetClose().Sub(k.GetOpen())
}

func (k KLineWindow) GetMaxChange() fixedpoint.Value {
	return k.GetHigh().Sub(k.GetLow())
}

func (k KLineWindow) GetAmplification() fixedpoint.Value {
	return k.GetMaxChange().Div(k.GetLow())
}

func (k KLineWindow) AllDrop() bool {
	for _, n := range k {
		if n.Direction() >= 0 {
			return false
		}
	}
	return true
}

func (k KLineWindow) AllRise() bool {
	for _, n := range k {
		if n.Direction() <= 0 {
			return false
		}
	}
	return true
}

func (k KLineWindow) GetTrend() int {
	o := k.GetOpen()
	c := k.GetClose()

	if c.Compare(o) > 0 {
		return 1
	} else if c.Compare(o) < 0 {
		return -1
	}
	return 0
}

func (k KLineWindow) Color() string {
	if k.GetTrend() > 0 {
		return GreenColor
	} else if k.GetTrend() < 0 {
		return RedColor
	}
	return GrayColor
}

// Mid price
func (k KLineWindow) Mid() fixedpoint.Value {
	return k.GetHigh().Add(k.GetLow()).Div(Two)
}

// BounceUp returns true if it's green candle with open and close near high price
func (k KLineWindow) BounceUp() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	return trend > 0 && k.GetOpen().Compare(mid) > 0 && k.GetClose().Compare(mid) > 0
}

// BounceDown returns true red candle with open and close near low price
func (k KLineWindow) BounceDown() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	return trend > 0 && k.GetOpen().Compare(mid) < 0 && k.GetClose().Compare(mid) < 0
}

func (k *KLineWindow) Add(line KLine) {
	*k = append(*k, line)
}

func (k KLineWindow) Take(size int) KLineWindow {
	return k[:size]
}

func (k KLineWindow) Tail(size int) KLineWindow {
	length := len(k)
	if length <= size {
		win := make(KLineWindow, length)
		copy(win, k)
		return win
	}

	win := make(KLineWindow, size)
	copy(win, k[length-size:])
	return win
}

// Truncate removes the old klines from the window
func (k *KLineWindow) Truncate(size int) {
	if len(*k) <= size {
		return
	}

	end := len(*k)
	start := end - size
	if start < 0 {
		start = 0
	}
	kn := (*k)[start:]
	*k = kn
}

func (k KLineWindow) GetBody() fixedpoint.Value {
	return k.GetChange()
}

func (k KLineWindow) GetThickness() fixedpoint.Value {
	out := k.GetChange().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLineWindow) GetUpperShadowRatio() fixedpoint.Value {
	out := k.GetUpperShadowHeight().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLineWindow) GetUpperShadowHeight() fixedpoint.Value {
	high := k.GetHigh()
	open := k.GetOpen()
	clos := k.GetClose()
	if open.Compare(clos) > 0 {
		return high.Sub(open)
	}
	return high.Sub(clos)
}

func (k KLineWindow) GetLowerShadowRatio() fixedpoint.Value {
	out := k.GetLowerShadowHeight().Div(k.GetMaxChange())
	if out.Sign() < 0 {
		return out.Neg()
	}
	return out
}

func (k KLineWindow) GetLowerShadowHeight() fixedpoint.Value {
	low := k.GetLow()
	open := k.GetOpen()
	clos := k.GetClose()
	if open.Compare(clos) < 0 {
		return open.Sub(low)
	}
	return clos.Sub(low)
}

func (k KLineWindow) SlackAttachment() slack.Attachment {
	var first KLine
	var end KLine
	var windowSize = len(k)
	if windowSize > 0 {
		first = k[0]
		end = k[windowSize-1]
	}

	return slack.Attachment{
		Text:  fmt.Sprintf("*%s* KLineWindow %s x %d", first.Symbol, first.Interval, windowSize),
		Color: k.Color(),
		Fields: []slack.AttachmentField{
			{Title: "Open", Value: util.FormatValue(k.GetOpen(), 2), Short: true},
			{Title: "High", Value: util.FormatValue(k.GetHigh(), 2), Short: true},
			{Title: "Low", Value: util.FormatValue(k.GetLow(), 2), Short: true},
			{Title: "Close", Value: util.FormatValue(k.GetClose(), 2), Short: true},
			{Title: "Mid", Value: util.FormatValue(k.Mid(), 2), Short: true},
			{
				Title: "Change",
				Value: util.FormatValue(k.GetChange(), 2),
				Short: true,
			},
			{
				Title: "Max Change",
				Value: util.FormatValue(k.GetMaxChange(), 2),
				Short: true,
			},
			{
				Title: "Thickness",
				Value: util.FormatValue(k.GetThickness(), 4),
				Short: true,
			},
			{
				Title: "UpperShadowRatio",
				Value: util.FormatValue(k.GetUpperShadowRatio(), 4),
				Short: true,
			},
			{
				Title: "LowerShadowRatio",
				Value: util.FormatValue(k.GetLowerShadowRatio(), 4),
				Short: true,
			},
		},
		Footer:     fmt.Sprintf("Since %s til %s", first.StartTime, end.EndTime),
		FooterIcon: "",
	}
}

type KLineCallback func(kline KLine)

type KValueType int

const (
	kOpUnknown KValueType = iota
	kOpenValue
	kCloseValue
	kHighValue
	kLowValue
	kVolumeValue
)

func (k *KLineWindow) High() Series {
	return &KLineSeries{
		lines: k,
		kv:    kHighValue,
	}
}

func (k *KLineWindow) Low() Series {
	return &KLineSeries{
		lines: k,
		kv:    kLowValue,
	}
}

func (k *KLineWindow) Open() Series {
	return &KLineSeries{
		lines: k,
		kv:    kOpenValue,
	}
}

func (k *KLineWindow) Close() Series {
	return &KLineSeries{
		lines: k,
		kv:    kCloseValue,
	}
}

func (k *KLineWindow) Volume() Series {
	return &KLineSeries{
		lines: k,
		kv:    kVolumeValue,
	}
}

type KLineSeries struct {
	lines *KLineWindow
	kv    KValueType
}

func (k *KLineSeries) Last() float64 {
	length := len(*k.lines)
	switch k.kv {
	case kOpenValue:
		return (*k.lines)[length-1].GetOpen().Float64()
	case kCloseValue:
		return (*k.lines)[length-1].GetClose().Float64()
	case kLowValue:
		return (*k.lines)[length-1].GetLow().Float64()
	case kHighValue:
		return (*k.lines)[length-1].GetHigh().Float64()
	case kVolumeValue:
		return (*k.lines)[length-1].Volume.Float64()
	}
	return 0
}

func (k *KLineSeries) Index(i int) float64 {
	length := len(*k.lines)
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	switch k.kv {
	case kOpenValue:
		return (*k.lines)[length-i-1].GetOpen().Float64()
	case kCloseValue:
		return (*k.lines)[length-i-1].GetClose().Float64()
	case kLowValue:
		return (*k.lines)[length-i-1].GetLow().Float64()
	case kHighValue:
		return (*k.lines)[length-i-1].GetHigh().Float64()
	case kVolumeValue:
		return (*k.lines)[length-i-1].Volume.Float64()
	}
	return 0
}

func (k *KLineSeries) Length() int {
	return len(*k.lines)
}

var _ Series = &KLineSeries{}
