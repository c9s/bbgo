package bbgo

import (
	"fmt"
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

func (k KLine) GetThickness() float64 {
	return k.GetChange() / k.GetMaxChange()
}

func (k KLine) GetChange() float64 {
	return k.GetClose() - k.GetOpen()
}

func (k KLine) String() string {
	return fmt.Sprintf("%s %s Open: % 14s Close: % 14s High: % 14s Low: % 14s Volume: % 13s Change: % 13f %s", k.Symbol, k.Interval, k.Open, k.Close, k.High, k.Low, k.Volume, k.GetChange(), k.Interval)
}

type KLineWindow []KLine

func (w KLineWindow) Len() int {
	return len(w)
}

func (w KLineWindow) GetOpen() float64 {
	return w[0].GetOpen()
}

func (w KLineWindow) GetClose() float64 {
	end := len(w) - 1
	return w[end].GetClose()
}

func (w KLineWindow) GetHigh() float64 {
	high := w.GetOpen()
	for _, line := range w {
		val := line.GetHigh()
		if val > high {
			high = val
		}
	}
	return high
}

func (w KLineWindow) GetLow() float64 {
	low := w.GetOpen()
	for _, line := range w {
		val := line.GetHigh()
		if val < low {
			low = val
		}
	}
	return low
}

func (w KLineWindow) GetChange() float64 {
	return w.GetClose() - w.GetOpen()
}

func (w KLineWindow) GetMaxChange() float64 {
	return w.GetHigh() - w.GetLow()
}

func (w *KLineWindow) Add(line KLine) {
	*w = append(*w, line)
}

func (w *KLineWindow) Truncate(size int) {
	if len(*w) <= size {
		return
	}

	end := len(*w) - 1
	start := end - size
	if start < 0 {
		start = 0
	}
	*w = (*w)[end-5 : end]
}

