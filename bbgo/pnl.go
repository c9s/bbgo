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
	Symbol             string
	StartTime          time.Time
	CurrentPrice       float64
	Trades             []types.Trade
	TradingFeeCurrency string
}

func (c *ProfitAndLossCalculator) AddTrade(trade types.Trade) {
	c.Trades = append(c.Trades, trade)
}

func (c *ProfitAndLossCalculator) SetCurrentPrice(price float64) {
	c.CurrentPrice = price
}

func (c *ProfitAndLossCalculator) Calculate() *ProfitAndLossReport {
	// copy trades, so that we can truncate it.
	var trades = c.Trades
	var bidVolume = 0.0
	var bidAmount = 0.0

	var askVolume = 0.0

	var feeUSD = 0.0
	var bidFeeUSD = 0.0
	var feeRate = 0.0015

	var currencyFees = map[string]float64{}

	for _, trade := range trades {
		if trade.Symbol == c.Symbol {
			if trade.IsBuyer {
				bidVolume += trade.Quantity
				bidAmount += trade.Price * trade.Quantity
			}

			// since we use USDT as the quote currency, we simply check if it matches the currency symbol
			if strings.HasPrefix(trade.Symbol, trade.FeeCurrency) {
				bidVolume -= trade.Fee
				feeUSD += trade.Price * trade.Fee
				if trade.IsBuyer {
					bidFeeUSD += trade.Price * trade.Fee
				}
			} else if trade.FeeCurrency == "USDT" {
				feeUSD += trade.Fee
				if trade.IsBuyer {
					bidFeeUSD += trade.Fee
				}
			}

		} else {
			if trade.FeeCurrency == c.TradingFeeCurrency {
				bidVolume -= trade.Fee
			}
		}

		if _, ok := currencyFees[trade.FeeCurrency]; !ok {
			currencyFees[trade.FeeCurrency] = 0.0
		}
		currencyFees[trade.FeeCurrency] += trade.Fee
	}

	log.Infof("average bid price = (total amount %f + total feeUSD %f) / volume %f", bidAmount, bidFeeUSD, bidVolume)
	profit := 0.0
	averageBidPrice := (bidAmount + bidFeeUSD) / bidVolume

	for _, t := range trades {
		if t.Symbol != c.Symbol {
			continue
		}

		if t.IsBuyer {
			continue
		}

		profit += (t.Price - averageBidPrice) * t.Quantity
		askVolume += t.Quantity
	}

	profit -= feeUSD

	stock := bidVolume - askVolume
	if stock > 0 {
		_ = feeRate
		// stockFee := c.CurrentPrice * feeRate * stock
		// profit += (c.CurrentPrice-averageBidPrice)*stock - stockFee
		// futureFee += stockFee
	}

	return &ProfitAndLossReport{
		Symbol:       c.Symbol,
		StartTime:    c.StartTime,
		CurrentPrice: c.CurrentPrice,
		NumTrades:    len(trades),

		BidVolume: bidVolume,
		AskVolume: askVolume,

		Stock:          stock,
		Profit:         profit,
		AverageBidCost: averageBidPrice,
		FeeUSD:         feeUSD,
		CurrencyFees:   currencyFees,
	}
}

type ProfitAndLossReport struct {
	CurrentPrice float64
	StartTime    time.Time
	Symbol       string

	NumTrades      int
	Profit         float64
	AverageBidCost float64
	BidVolume      float64
	AskVolume      float64
	FeeUSD         float64
	Stock          float64
	CurrencyFees   map[string]float64
}

func (report ProfitAndLossReport) Print() {
	log.Infof("trades since: %v", report.StartTime)
	log.Infof("average bid cost: %s", USD.FormatMoneyFloat64(report.AverageBidCost))
	log.Infof("total bid volume: %f", report.BidVolume)
	log.Infof("total ask volume: %f", report.AskVolume)
	log.Infof("stock: %f", report.Stock)
	log.Infof("fee (USD): %f", report.FeeUSD)
	log.Infof("current price: %s", USD.FormatMoneyFloat64(report.CurrentPrice))
	log.Infof("profit: %s", USD.FormatMoneyFloat64(report.Profit))
	log.Infof("currency fees:")
	for currency, fee := range report.CurrencyFees {
		log.Infof(" - %s: %f", currency, fee)
	}
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
			{Title: "Symbol", Value: report.Symbol, Short: true},
			{Title: "Profit", Value: USD.FormatMoney(report.Profit), Short: true},
			{Title: "Fee (USD)", Value: USD.FormatMoney(report.FeeUSD), Short: true},
			{Title: "Current Price", Value: USD.FormatMoney(report.CurrentPrice), Short: true},
			{Title: "Average Bid Cost", Value: USD.FormatMoney(report.AverageBidCost), Short: true},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true},
			{Title: "Stock", Value: strconv.FormatFloat(report.Stock, 'f', 8, 64), Short: true},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
