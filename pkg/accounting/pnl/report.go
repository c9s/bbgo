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
	Market       types.Market

	NumTrades        int
	Profit           float64
	UnrealizedProfit float64
	AverageBidCost   float64
	BuyVolume        float64
	SellVolume       float64
	FeeInUSD         float64
	Stock            float64
	CurrencyFees     map[string]float64
}

func (report AverageCostPnlReport) Print() {
	log.Infof("TRADES SINCE: %v", report.StartTime)
	log.Infof("NUMBER OF TRADES: %d", report.NumTrades)
	log.Infof("AVERAGE COST: %s", types.USD.FormatMoneyFloat64(report.AverageBidCost))
	log.Infof("TOTAL BUY VOLUME: %f", report.BuyVolume)
	log.Infof("TOTAL SELL VOLUME: %f", report.SellVolume)
	log.Infof("STOCK: %f", report.Stock)
	log.Infof("FEE (USD): %f", report.FeeInUSD)
	log.Infof("CURRENT PRICE: %s", types.USD.FormatMoneyFloat64(report.CurrentPrice))
	log.Infof("CURRENCY FEES:")
	for currency, fee := range report.CurrencyFees {
		log.Infof(" - %s: %f", currency, fee)
	}
	log.Infof("PROFIT: %s", types.USD.FormatMoneyFloat64(report.Profit))
	log.Infof("UNREALIZED PROFIT: %s", types.USD.FormatMoneyFloat64(report.UnrealizedProfit))
}

func (report AverageCostPnlReport) SlackAttachment() slack.Attachment {
	var color = slackstyle.Red

	if report.UnrealizedProfit > 0 {
		color = slackstyle.Green
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
			{Title: "Current Price", Value: report.Market.FormatPrice(report.CurrentPrice), Short: true},
			{Title: "Average Cost", Value: report.Market.FormatPrice(report.AverageBidCost), Short: true},
			{Title: "Fee (USD)", Value: types.USD.FormatMoney(report.FeeInUSD), Short: true},
			{Title: "Stock", Value: strconv.FormatFloat(report.Stock, 'f', 8, 64), Short: true},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
