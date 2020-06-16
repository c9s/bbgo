package bbgo

import (
	"fmt"
	"github.com/slack-go/slack"
	"math"
	"strconv"
)

type KLineEvent struct {
	EventBase
	Symbol string `json:"s"`
	KLine  *KLine `json:"k,omitempty"`
}

type KLine struct {
	StartTime int64 `json:"t"`
	EndTime   int64 `json:"T"`

	Symbol   string `json:"s"`
	Interval string `json:"i"`

	Open        string `json:"o"`
	Close       string `json:"c"`
	High        string `json:"h"`
	Low         string `json:"l"`
	Volume      string `json:"V"` // taker buy base asset volume (like 10 BTC)
	QuoteVolume string `json:"Q"` // taker buy quote asset volume (like 1000USDT)

	LastTradeID    int  `json:"L"`
	NumberOfTrades int  `json:"n"`
	Closed         bool `json:"x"`
}

func (k KLine) Mid() float64 {
	return (k.GetHigh() + k.GetLow()) / 2
}

// green candle with open and close near high price
func (k KLine) BounceUp() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	thickness := k.GetThickness()
	return trend > 0 && k.GetOpen() > mid && k.GetClose() > mid && thickness < (1.0/4.0)
}

// red candle with open and close near low price
func (k KLine) BounceDown() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	thickness := k.GetThickness()
	return trend > 0 && k.GetOpen() < mid && k.GetClose() < mid && thickness < (1.0/4.0)
}

func (k KLine) GetTrend() int {
	o := k.GetOpen()
	c := k.GetClose()

	if c > o {
		return 1
	} else if c < o {
		return -1
	}
	return 0
}

func (k KLine) GetHigh() float64 {
	return MustParseFloat(k.High)
}

func (k KLine) GetLow() float64 {
	return MustParseFloat(k.Low)
}

func (k KLine) GetOpen() float64 {
	return MustParseFloat(k.Open)
}

func (k KLine) GetClose() float64 {
	return MustParseFloat(k.Close)
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
	low := k.GetLow()
	if k.GetOpen() < k.GetClose() {
		return k.GetOpen() - low
	}
	return k.GetClose() - low
}

// GetBody returns the height of the candle real body
func (k KLine) GetBody() float64 {
	return k.GetChange()
}

func (k KLine) GetChange() float64 {
	return k.GetClose() - k.GetOpen()
}

func (k KLine) String() string {
	return fmt.Sprintf("%s %s Open: % 14s Close: % 14s High: % 14s Low: % 14s Volume: % 15s Change: % 11f Max Change: % 11f", k.Symbol, k.Interval, k.Open, k.Close, k.High, k.Low, k.Volume, k.GetChange(), k.GetMaxChange())
}


func (k KLine) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Text: "KLine",
		Fields: []slack.AttachmentField{
			{
				Title: "Open",
				Value: k.Open,
				Short: true,
			},
			{
				Title: "Close",
				Value: k.Close,
				Short: true,
			},
			{
				Title: "High",
				Value: k.High,
				Short: true,
			},
			{
				Title: "Low",
				Value: k.Low,
				Short: true,
			},
			{
				Title: "Change",
				Value: formatVolume(k.GetChange()),
				Short: true,
			},
			{
				Title: "Max Change",
				Value: formatVolume(k.GetMaxChange()),
				Short: true,
			},
		},
		Footer:     "",
		FooterIcon: "",
	}
}



type KLineWindow []KLine

func (k KLineWindow) Len() int {
	return len(k)
}

func (k KLineWindow) GetOpen() float64 {
	return k[0].GetOpen()
}

func (k KLineWindow) GetClose() float64 {
	end := len(k) - 1
	return k[end].GetClose()
}

func (k KLineWindow) GetHigh() float64 {
	high := k.GetOpen()
	for _, line := range k {
		val := line.GetHigh()
		if val > high {
			high = val
		}
	}
	return high
}

func (k KLineWindow) GetLow() float64 {
	low := k.GetOpen()
	for _, line := range k {
		val := line.GetHigh()
		if val < low {
			low = val
		}
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
		if n.GetTrend() >= 0 {
			return false
		}
	}
	return true
}

func (k KLineWindow) AllRise() bool {
	for _, n := range k {
		if n.GetTrend() <= 0 {
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

func (k KLineWindow) Mid() float64 {
	return k.GetHigh() - k.GetLow()/2
}

// green candle with open and close near high price
func (k KLineWindow) BounceUp() bool {
	mid := k.Mid()
	trend := k.GetTrend()
	return trend > 0 && k.GetOpen() > mid && k.GetClose() > mid
}

// red candle with open and close near low price
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
	if len(k) <= size {
		return k[:]
	}
	return k[len(k) - size:]
}

func (k *KLineWindow) Truncate(size int) {
	if len(*k) <= size {
		return
	}

	end := len(*k) - 1
	start := end - size
	if start < 0 {
		start = 0
	}
	*k = (*k)[end-5 : end]
}


func formatVolume(val float64) string {
	return strconv.FormatFloat(val, 'f', 6, 64)
}

