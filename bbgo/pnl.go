package bbgo

import (
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"strconv"
	"strings"
	"time"
)

type ProfitAndLossCalculator struct {
	Symbol       string
	StartTime    time.Time
	CurrentPrice float64
	Trades       []types.Trade

	CurrencyPrice map[string]float64
}

func (c *ProfitAndLossCalculator) AddTrade(trade types.Trade) {
	c.Trades = append(c.Trades, trade)
}

func (c *ProfitAndLossCalculator) SetCurrencyPrice(symbol string, price float64) {
	if c.CurrencyPrice == nil {
		c.CurrencyPrice = map[string]float64{}
	}

	c.CurrencyPrice[symbol] = price
}

func (c *ProfitAndLossCalculator) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}

func (c *ProfitAndLossCalculator) Calculate() *ProfitAndLossReport {
	// copy trades, so that we can truncate it.
	var trades = c.Trades
	var bidVolume = 0.0
	var bidAmount = 0.0
	var bidFee = 0.0

	var askVolume = 0.0
	var askFee = 0.0
	var feeRate = 0.001

	for _, t := range trades {
		if t.IsBuyer {
			bidVolume += t.Quantity
			bidAmount += t.Price * t.Quantity

			// since we use USDT as the quote currency, we simply check if it matches the currency symbol
			if strings.HasPrefix(t.Symbol, t.FeeCurrency) {
				bidVolume -= t.Fee
				bidFee += t.Price * t.Fee
			} else if t.FeeCurrency == "USDT" {
				bidFee += t.Fee
			}
		}
	}

	log.Infof("average bid price = (total amount %f + total fee %f) / volume %f", bidAmount, bidFee, bidVolume)
	profit := 0.0
	averageBidPrice := (bidAmount + bidFee) / bidVolume

	for _, t := range trades {
		if !t.IsBuyer {
			profit += (t.Price - averageBidPrice) * t.Quantity
			askVolume += t.Quantity

			// since we use USDT as the quote currency, we simply check if it matches the currency symbol
			if strings.HasPrefix(t.Symbol, t.FeeCurrency) {
				askFee += t.Price * t.Fee
			} else if t.FeeCurrency == "USDT" {
				askFee += t.Fee
			}
		}
	}

	profit -= askFee

	stock := bidVolume - askVolume
	futureFee := 0.0
	if stock > 0 {
		_ = feeRate
		// stockFee := c.CurrentPrice * feeRate * stock
		// profit += (c.CurrentPrice-averageBidPrice)*stock - stockFee
		// futureFee += stockFee
	}

	fee := bidFee + askFee + futureFee

	return &ProfitAndLossReport{
		Symbol:       c.Symbol,
		StartTime:    c.StartTime,
		CurrentPrice: c.CurrentPrice,
		NumTrades:    len(trades),

		BidVolume: bidVolume,
		AskVolume: askVolume,

		Profit:          profit,
		AverageBidPrice: averageBidPrice,
		Fee:             fee,
	}
}

type ProfitAndLossReport struct {
	CurrentPrice float64
	StartTime    time.Time
	Symbol       string

	NumTrades       int
	Profit          float64
	AverageBidPrice float64
	BidVolume float64
	AskVolume float64
	Fee             float64
}

func (report ProfitAndLossReport) Print() {
	log.Infof("trades since: %v", report.StartTime)
	log.Infof("average bid price: %s", USD.FormatMoneyFloat64(report.AverageBidPrice))
	log.Infof("total bid volume: %f", report.BidVolume)
	log.Infof("total ask volume: %f", report.AskVolume)
	log.Infof("current price: %s", USD.FormatMoneyFloat64(report.CurrentPrice))
	log.Infof("overall profit: %s", USD.FormatMoneyFloat64(report.Profit))
}

func (report ProfitAndLossReport) SlackAttachment() slack.Attachment {
	var color = ""
	if report.Profit > 0 {
		color = slackstyle.Green
	} else {
		color = slackstyle.Red
	}

	market, ok := types.FindMarket(report.Symbol)
	if !ok {
		return slack.Attachment{}
	}

	_ = market

	return slack.Attachment{
		Title: "Profit and Loss report of " + report.Symbol,
		Color: color,
		// Pretext:       "",
		// Text:          "",
		Fields: []slack.AttachmentField{
			{Title: "Symbol", Value: report.Symbol, Short: true,},
			{Title: "Profit", Value: USD.FormatMoney(report.Profit), Short: true,},
			{Title: "Current Price", Value: USD.FormatMoney(report.CurrentPrice), Short: true,},
			{Title: "Average Bid Price", Value: USD.FormatMoney(report.AverageBidPrice), Short: true,},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true,},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
