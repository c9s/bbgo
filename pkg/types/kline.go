package types

import (
	"fmt"
	"math"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/util"
)

type Direction int

const DirectionUp = 1
const DirectionNone = 0
const DirectionDown = -1

type KLineOrWindow interface {
	GetInterval() string
	Direction() Direction
	GetChange() float64
	GetMaxChange() float64
	GetThickness() float64

	Mid() float64
	GetOpen() float64
	GetClose() float64
	GetHigh() float64
	GetLow() float64

	BounceUp() bool
	BounceDown() bool
	GetUpperShadowRatio() float64
	GetLowerShadowRatio() float64

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

	StartTime time.Time `json:"startTime" db:"start_time"`
	EndTime   time.Time `json:"endTime" db:"end_time"`

	Interval Interval `json:"interval" db:"interval"`

	Open                     float64 `json:"open" db:"open"`
	Close                    float64 `json:"close" db:"close"`
	High                     float64 `json:"high" db:"high"`
	Low                      float64 `json:"low" db:"low"`
	Volume                   float64 `json:"volume" db:"volume"`
	QuoteVolume              float64 `json:"quoteVolume" db:"quote_volume"`
	TakerBuyBaseAssetVolume  float64 `json:"takerBuyBaseAssetVolume" db:"taker_buy_base_volume"`
	TakerBuyQuoteAssetVolume float64 `json:"takerBuyQuoteAssetVolume" db:"taker_buy_quote_volume"`

	LastTradeID    uint64 `json:"lastTradeID" db:"last_trade_id"`
	NumberOfTrades uint64 `json:"numberOfTrades" db:"num_trades"`
	Closed         bool   `json:"closed" db:"closed"`
}

func (k KLine) GetStartTime() time.Time {
	return k.StartTime
}

func (k KLine) GetEndTime() time.Time {
	return k.EndTime
}

func (k KLine) GetInterval() Interval {
	return k.Interval
}

func (k KLine) Mid() float64 {
	return (k.High + k.Low) / 2
}

// green candle with open and close near high price
func (k KLine) BounceUp() bool {
	mid := k.Mid()
	trend := k.Direction()
	return trend > 0 && k.Open > mid && k.Close > mid
}

// red candle with open and close near low price
func (k KLine) BounceDown() bool {
	mid := k.Mid()
	trend := k.Direction()
	return trend > 0 && k.Open < mid && k.Close < mid
}

func (k KLine) Direction() Direction {
	o := k.GetOpen()
	c := k.GetClose()

	if c > o {
		return DirectionUp
	} else if c < o {
		return DirectionDown
	}
	return DirectionNone
}

func (k KLine) GetHigh() float64 {
	return k.High
}

func (k KLine) GetLow() float64 {
	return k.Low
}

func (k KLine) GetOpen() float64 {
	return k.Open
}

func (k KLine) GetClose() float64 {
	return k.Close
}

func (k KLine) GetMaxChange() float64 {
	return k.GetHigh() - k.GetLow()
}

// GetThickness returns the thickness of the kline. 1 => thick, 0.1 => thin
func (k KLine) GetThickness() float64 {
	return math.Abs(k.GetChange()) / math.Abs(k.GetMaxChange())
}

func (k KLine) GetUpperShadowRatio() float64 {
	return k.GetUpperShadowHeight() / math.Abs(k.GetMaxChange())
}

func (k KLine) GetUpperShadowHeight() float64 {
	high := k.GetHigh()
	if k.GetOpen() > k.GetClose() {
		return high - k.GetOpen()
	}
	return high - k.GetClose()
}

func (k KLine) GetLowerShadowRatio() float64 {
	return k.GetLowerShadowHeight() / math.Abs(k.GetMaxChange())
}

func (k KLine) GetLowerShadowHeight() float64 {
	low := k.Low
	if k.Open < k.Close {
		return k.Open - low
	}
	return k.Close - low
}

// GetBody returns the height of the candle real body
func (k KLine) GetBody() float64 {
	return k.GetChange()
}

// GetChange returns Close price - Open price.
func (k KLine) GetChange() float64 {
	return k.Close - k.Open
}

func (k KLine) String() string {
	return fmt.Sprintf("%s %s %s %s O: %.4f H: %.4f L: %.4f C: %.4f CHG: %.4f MAXCHG: %.4f V: %.4f QV: %.2f TBBV: %.2f",
		k.Exchange.String(),
		k.StartTime.Format("2006-01-02 15:04"),
		k.Symbol, k.Interval, k.Open, k.High, k.Low, k.Close, k.GetChange(), k.GetMaxChange(), k.Volume, k.QuoteVolume, k.TakerBuyBaseAssetVolume)
}

func (k KLine) Color() string {
	if k.Direction() > 0 {
		return GreenColor
	} else if k.Direction() < 0 {
		return RedColor
	}
	return GrayColor
}

func (k KLine) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Text:  fmt.Sprintf("*%s* KLine %s", k.Symbol, k.Interval),
		Color: k.Color(),
		Fields: []slack.AttachmentField{
			{Title: "Open", Value: util.FormatFloat(k.Open, 2), Short: true},
			{Title: "High", Value: util.FormatFloat(k.High, 2), Short: true},
			{Title: "Low", Value: util.FormatFloat(k.Low, 2), Short: true},
			{Title: "Close", Value: util.FormatFloat(k.Close, 2), Short: true},
			{Title: "Mid", Value: util.FormatFloat(k.Mid(), 2), Short: true},
			{Title: "Change", Value: util.FormatFloat(k.GetChange(), 2), Short: true},
			{Title: "Volume", Value: util.FormatFloat(k.Volume, 2), Short: true},
			{Title: "Taker Buy Base Volume", Value: util.FormatFloat(k.TakerBuyBaseAssetVolume, 2), Short: true},
			{Title: "Taker Buy Quote Volume", Value: util.FormatFloat(k.TakerBuyQuoteAssetVolume, 2), Short: true},
			{Title: "Max Change", Value: util.FormatFloat(k.GetMaxChange(), 2), Short: true},
			{
				Title: "Thickness",
				Value: util.FormatFloat(k.GetThickness(), 4),
				Short: true,
			},
			{
				Title: "UpperShadowRatio",
				Value: util.FormatFloat(k.GetUpperShadowRatio(), 4),
				Short: true,
			},
			{
				Title: "LowerShadowRatio",
				Value: util.FormatFloat(k.GetLowerShadowRatio(), 4),
				Short: true,
			},
		},
		Footer:     "",
		FooterIcon: "",
	}
}

type KLineWindow []KLine

// ReduceClose reduces the closed prices
func (k KLineWindow) ReduceClose() float64 {
	s := 0.0

	for _, kline := range k {
		s += kline.GetClose()
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

func (k KLineWindow) GetOpen() float64 {
	return k.First().GetOpen()
}

func (k KLineWindow) GetClose() float64 {
	end := len(k) - 1
	return k[end].GetClose()
}

func (k KLineWindow) GetHigh() float64 {
	high := k.First().GetHigh()
	for _, line := range k {
		high = math.Max(high, line.GetHigh())
	}

	return high
}

func (k KLineWindow) GetLow() float64 {
	low := k.First().GetLow()
	for _, line := range k {
		low = math.Min(low, line.GetLow())
	}

	return low
}

func (k KLineWindow) GetChange() float64 {
	return k.GetClose() - k.GetOpen()
}

func (k KLineWindow) GetMaxChange() float64 {
	return k.GetHigh() - k.GetLow()
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

	if c > o {
		return 1
	} else if c < o {
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
func (k KLineWindow) Mid() float64 {
	return (k.GetHigh() + k.GetLow()) / 2.0
}

// BounceUp returns true if it's green candle with open and close near high price
func (k KLineWindow) BounceUp() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	return trend > 0 && k.GetOpen() > mid && k.GetClose() > mid
}

// BounceDown returns true red candle with open and close near low price
func (k KLineWindow) BounceDown() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	return trend > 0 && k.GetOpen() < mid && k.GetClose() < mid
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

func (k KLineWindow) GetBody() float64 {
	return k.GetChange()
}

func (k KLineWindow) GetThickness() float64 {
	return math.Abs(k.GetChange()) / math.Abs(k.GetMaxChange())
}

func (k KLineWindow) GetUpperShadowRatio() float64 {
	return k.GetUpperShadowHeight() / math.Abs(k.GetMaxChange())
}

func (k KLineWindow) GetUpperShadowHeight() float64 {
	high := k.GetHigh()
	if k.GetOpen() > k.GetClose() {
		return high - k.GetOpen()
	}
	return high - k.GetClose()
}

func (k KLineWindow) GetLowerShadowRatio() float64 {
	return k.GetLowerShadowHeight() / math.Abs(k.GetMaxChange())
}

func (k KLineWindow) GetLowerShadowHeight() float64 {
	low := k.GetLow()
	if k.GetOpen() < k.GetClose() {
		return k.GetOpen() - low
	}
	return k.GetClose() - low
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
			{Title: "Open", Value: util.FormatFloat(k.GetOpen(), 2), Short: true},
			{Title: "High", Value: util.FormatFloat(k.GetHigh(), 2), Short: true},
			{Title: "Low", Value: util.FormatFloat(k.GetLow(), 2), Short: true},
			{Title: "Close", Value: util.FormatFloat(k.GetClose(), 2), Short: true},
			{Title: "Mid", Value: util.FormatFloat(k.Mid(), 2), Short: true},
			{
				Title: "Change",
				Value: util.FormatFloat(k.GetChange(), 2),
				Short: true,
			},
			{
				Title: "Max Change",
				Value: util.FormatFloat(k.GetMaxChange(), 2),
				Short: true,
			},
			{
				Title: "Thickness",
				Value: util.FormatFloat(k.GetThickness(), 4),
				Short: true,
			},
			{
				Title: "UpperShadowRatio",
				Value: util.FormatFloat(k.GetUpperShadowRatio(), 4),
				Short: true,
			},
			{
				Title: "LowerShadowRatio",
				Value: util.FormatFloat(k.GetLowerShadowRatio(), 4),
				Short: true,
			},
		},
		Footer:     fmt.Sprintf("Since %s til %s", first.StartTime, end.EndTime),
		FooterIcon: "",
	}
}

type KLineCallback func(kline KLine)
