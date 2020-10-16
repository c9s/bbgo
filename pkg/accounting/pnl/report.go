package pnl

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/types"
)

type AverageCostPnlReport struct {
	CurrentPrice float64
	StartTime    time.Time
	Symbol       string

	NumTrades        int
	Profit           float64
	UnrealizedProfit float64
	AverageBidCost   float64
	BidVolume        float64
	AskVolume        float64
	FeeUSD           float64
	Stock            float64
	CurrencyFees     map[string]float64
}

func (report AverageCostPnlReport) Print() {
	log.Infof("trades since: %v", report.StartTime)
	log.Infof("average bid cost: %s", types.USD.FormatMoneyFloat64(report.AverageBidCost))
	log.Infof("total bid volume: %f", report.BidVolume)
	log.Infof("total ask volume: %f", report.AskVolume)
	log.Infof("stock: %f", report.Stock)
	log.Infof("fee (USD): %f", report.FeeUSD)
	log.Infof("current price: %s", types.USD.FormatMoneyFloat64(report.CurrentPrice))
	log.Infof("profit: %s", types.USD.FormatMoneyFloat64(report.Profit))
	log.Infof("unrealized profit: %s", types.USD.FormatMoneyFloat64(report.UnrealizedProfit))
	log.Infof("currency fees:")
	for currency, fee := range report.CurrencyFees {
		log.Infof(" - %s: %f", currency, fee)
	}
}

func (report AverageCostPnlReport) SlackAttachment() slack.Attachment {
	var color = slackstyle.Red

	if report.UnrealizedProfit > 0 {
		color = slackstyle.Green
	}

	market, ok := types.FindMarket(report.Symbol)
	if !ok {
		return slack.Attachment{}
	}

	return slack.Attachment{
		Title: report.Symbol + " Profit and Loss report",
		Text:  "Profit " + types.USD.FormatMoney(report.Profit),
		Color: color,
		// Pretext:       "",
		// Text:          "",
		Fields: []slack.AttachmentField{
			{Title: "Profit", Value: types.USD.FormatMoney(report.Profit)},
			{Title: "Unrealized Profit", Value: types.USD.FormatMoney(report.UnrealizedProfit)},
			{Title: "Current Price", Value: market.FormatPrice(report.CurrentPrice), Short: true},
			{Title: "Average Cost", Value: market.FormatPrice(report.AverageBidCost), Short: true},
			{Title: "Fee (USD)", Value: types.USD.FormatMoney(report.FeeUSD), Short: true},
			{Title: "Stock", Value: strconv.FormatFloat(report.Stock, 'f', 8, 64), Short: true},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
