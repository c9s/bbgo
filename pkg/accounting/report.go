package accounting

import (
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitAndLossReport struct {
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

func (report ProfitAndLossReport) Print() {
	logrus.Infof("trades since: %v", report.StartTime)
	logrus.Infof("average bid cost: %s", types.USD.FormatMoneyFloat64(report.AverageBidCost))
	logrus.Infof("total bid volume: %f", report.BidVolume)
	logrus.Infof("total ask volume: %f", report.AskVolume)
	logrus.Infof("stock: %f", report.Stock)
	logrus.Infof("fee (USD): %f", report.FeeUSD)
	logrus.Infof("current price: %s", types.USD.FormatMoneyFloat64(report.CurrentPrice))
	logrus.Infof("profit: %s", types.USD.FormatMoneyFloat64(report.Profit))
	logrus.Infof("unrealized profit: %s", types.USD.FormatMoneyFloat64(report.UnrealizedProfit))
	logrus.Infof("currency fees:")
	for currency, fee := range report.CurrencyFees {
		logrus.Infof(" - %s: %f", currency, fee)
	}
}

func (report ProfitAndLossReport) SlackAttachment() slack.Attachment {
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
