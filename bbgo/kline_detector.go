package bbgo

import (
	"fmt"
	"github.com/adshao/go-binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/slack-go/slack"
	"math"
	"strconv"
)

const epsilon = 0.0000001

func NotZero(v float64) bool {
	return math.Abs(v) > epsilon
}

type KLineDetector struct {
	Name     string `json:"name"`
	Interval string `json:"interval"`

	// MinMaxPriceChange is the minimal max price change trigger
	MinMaxPriceChange float64 `json:"minPriceChange"`

	// MaxMaxPriceChange is the max - max price change trigger
	MaxMaxPriceChange float64 `json:"maxPriceChange"`

	EnableMinThickness bool    `json:"enableMinThickness"`
	MinThickness       float64 `json:"minThickness"`

	EnableMaxShadowRatio bool    `json:"enableMaxShadowRatio"`
	MaxShadowRatio       float64 `json:"maxShadowRatio"`

	EnableLookBack bool `json:"enableLookBack"`
	LookBackFrames int  `json:"lookBackFrames"`

	MinProfitPriceTick float64 `json:"minProfitPriceTick"`

	DelayMilliseconds int  `json:"delayMsec"`
	Stop              bool `json:"stop"`
}

func (d *KLineDetector) SlackAttachment() slack.Attachment {
	var name = "Detector "

	if len(d.Name) > 0 {
		name += " " + d.Name
	}

	name += fmt.Sprintf(" %s", d.Interval)

	if d.EnableLookBack {
		name += fmt.Sprintf(" x %d", d.LookBackFrames)
	}

	var maxPriceChangeRange = fmt.Sprintf("%.2f ~ NO LIMIT", d.MinMaxPriceChange)
	if NotZero(d.MaxMaxPriceChange) {
		maxPriceChangeRange = fmt.Sprintf("%.2f ~ %.2f", d.MinMaxPriceChange, d.MaxMaxPriceChange)
	}
	name += " MaxMaxPriceChange " + maxPriceChangeRange

	var fields = []slack.AttachmentField{
		{
			Title: "Interval",
			Value: d.Interval,
			Short: true,
		},
	}

	if d.EnableMinThickness && NotZero(d.MinThickness) {
		fields = append(fields, slack.AttachmentField{
			Title: "MinThickness",
			Value: formatFloat(d.MinThickness, 4),
			Short: true,
		})
	}

	if d.EnableMaxShadowRatio && NotZero(d.MaxShadowRatio) {
		fields = append(fields, slack.AttachmentField{
			Title: "MaxShadowRatio",
			Value: formatFloat(d.MaxShadowRatio, 4),
			Short: true,
		})
	}

	if d.EnableLookBack {
		fields = append(fields, slack.AttachmentField{
			Title: "LookBackFrames",
			Value: strconv.Itoa(d.LookBackFrames),
			Short: true,
		})
	}

	return slack.Attachment{
		Color:      "",
		Fallback:   "",
		ID:         0,
		Title:      name,
		Pretext:    "",
		Text:       "",
		Fields:     fields,
		Footer:     "",
		FooterIcon: "",
		Ts:         "",
	}

}

func (d *KLineDetector) String() string {
	var name = fmt.Sprintf("Detector %s (%f < x < %f)", d.Interval, d.MinMaxPriceChange, d.MaxMaxPriceChange)

	if d.EnableMinThickness {
		name += fmt.Sprintf(" [MinThickness: %f]", d.MinThickness)
	}

	if d.EnableLookBack {
		name += fmt.Sprintf(" [LookBack: %d]", d.LookBackFrames)
	}
	if d.EnableMaxShadowRatio {
		name += fmt.Sprintf(" [MaxShadowRatio: %f]", d.MaxShadowRatio)
	}

	return name
}

func (d *KLineDetector) NewOrder(e *KLineEvent, tradingCtx *TradingContext) *Order {
	var kline types.KLine = e.KLine
	if d.EnableLookBack {
		klineWindow := tradingCtx.KLineWindows[e.KLine.Interval]
		if len(klineWindow) >= d.LookBackFrames {
			kline = klineWindow.Tail(d.LookBackFrames)
		}
	}

	var trend = kline.GetTrend()

	var side binance.SideType
	if trend < 0 {
		side = binance.SideTypeBuy
	} else if trend > 0 {
		side = binance.SideTypeSell
	}

	var volume = tradingCtx.Market.FormatVolume(VolumeByPriceChange(tradingCtx.Market, kline.GetClose(), kline.GetChange(), side))
	return &Order{
		Symbol:    e.KLine.Symbol,
		Type:      binance.OrderTypeMarket,
		Side:      side,
		VolumeStr: volume,
	}
}

func (d *KLineDetector) Detect(e *KLineEvent, tradingCtx *TradingContext) (reason string, kline types.KLine, ok bool) {
	kline = e.KLine

	// if the 3m trend is drop, do not buy, let 5m window handle it.
	if d.EnableLookBack {
		klineWindow := tradingCtx.KLineWindows[e.KLine.Interval]
		if len(klineWindow) >= d.LookBackFrames {
			kline = klineWindow.Tail(d.LookBackFrames)
		}
		/*
			if lookbackKline.AllDrop() {
				trader.Infof("1m window all drop down (%d frames), do not buy: %+v", d.LookBackFrames, klineWindow)
			} else if lookbackKline.AllRise() {
				trader.Infof("1m window all rise up (%d frames), do not sell: %+v", d.LookBackFrames, klineWindow)
			}
		*/
	}

	var maxChange = math.Abs(kline.GetMaxChange())

	if maxChange < d.MinMaxPriceChange {
		return "", kline, false
	}

	if NotZero(d.MaxMaxPriceChange) && maxChange > d.MaxMaxPriceChange {
		return fmt.Sprintf("exceeded max price change %.4f > %.4f", maxChange, d.MaxMaxPriceChange), kline, false
	}

	if d.EnableMinThickness {
		if kline.GetThickness() < d.MinThickness {
			return fmt.Sprintf("kline too thin. %.4f < min kline thickness %.4f", kline.GetThickness(), d.MinThickness), kline, false
		}
	}

	var trend = kline.GetTrend()
	if d.EnableMaxShadowRatio {
		if trend > 0 {
			if kline.GetUpperShadowRatio() > d.MaxShadowRatio {
				return fmt.Sprintf("kline upper shadow ratio too high. %.4f > %.4f (MaxShadowRatio)", kline.GetUpperShadowRatio(), d.MaxShadowRatio), kline, false
			}
		} else if trend < 0 {
			if kline.GetLowerShadowRatio() > d.MaxShadowRatio {
				return fmt.Sprintf("kline lower shadow ratio too high. %.4f > %.4f (MaxShadowRatio)", kline.GetLowerShadowRatio(), d.MaxShadowRatio), kline, false
			}
		}
	}

	if trend > 0 && kline.BounceUp() { // trend up, ignore bounce up

		return fmt.Sprintf("bounce up, do not sell, kline mid: %.4f", kline.Mid()), kline, false

	} else if trend < 0 && kline.BounceDown() { // trend down, ignore bounce down

		return fmt.Sprintf("bounce down, do not buy, kline mid: %.4f", kline.Mid()), kline, false

	}

	if NotZero(d.MinProfitPriceTick) {

		// do not buy too early if it's greater than the average bid price + min profit tick
		if trend < 0 && kline.GetClose() > (tradingCtx.AverageBidPrice-d.MinProfitPriceTick) {
			return fmt.Sprintf("price %f is greater than the average price + min profit tick %f", kline.GetClose(), tradingCtx.AverageBidPrice - d.MinProfitPriceTick), kline, false
		}

		// do not sell too early if it's less than the average bid price + min profit tick
		if trend > 0 && kline.GetClose() < (tradingCtx.AverageBidPrice+d.MinProfitPriceTick) {
			return fmt.Sprintf("price %f is less than the average price + min profit tick %f", kline.GetClose(), tradingCtx.AverageBidPrice + d.MinProfitPriceTick), kline, false
		}

	}

	/*
			if toPrice(kline.GetClose()) == toPrice(kline.GetLow()) {
			return fmt.Sprintf("close near the lowest price, the price might continue to drop."), false
		}

	*/

	return "", kline, true
}
