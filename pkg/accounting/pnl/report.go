package pnl

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/types"
)

type AverageCostPnlReport struct {
	LastPrice fixedpoint.Value      `json:"lastPrice"`
	StartTime time.Time    `json:"startTime"`
	Symbol    string       `json:"symbol"`
	Market    types.Market `json:"market"`

	NumTrades        int                `json:"numTrades"`
	Profit           fixedpoint.Value   `json:"profit"`
	NetProfit        fixedpoint.Value   `json:"netProfit"`
	UnrealizedProfit fixedpoint.Value   `json:"unrealizedProfit"`
	AverageCost      fixedpoint.Value            `json:"averageCost"`
	BuyVolume        fixedpoint.Value            `json:"buyVolume,omitempty"`
	SellVolume       fixedpoint.Value            `json:"sellVolume,omitempty"`
	FeeInUSD         fixedpoint.Value            `json:"feeInUSD"`
	Stock            fixedpoint.Value            `json:"stock"`
	CurrencyFees     map[string]fixedpoint.Value `json:"currencyFees"`
}

func (report *AverageCostPnlReport) JSON() ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func (report AverageCostPnlReport) Print() {
	log.Infof("TRADES SINCE: %v", report.StartTime)
	log.Infof("NUMBER OF TRADES: %d", report.NumTrades)
	log.Infof("AVERAGE COST: %s", types.USD.FormatMoney(report.AverageCost))
	log.Infof("TOTAL BUY VOLUME: %v", report.BuyVolume)
	log.Infof("TOTAL SELL VOLUME: %v", report.SellVolume)
	log.Infof("STOCK: %s", report.Stock.String())

	// FIXME:
	// log.Infof("FEE (USD): %f", report.FeeInUSD)
	log.Infof("CURRENT PRICE: %s", types.USD.FormatMoney(report.LastPrice))
	log.Infof("CURRENCY FEES:")
	for currency, fee := range report.CurrencyFees {
		log.Infof(" - %s: %s", currency, fee.String())
	}
	log.Infof("PROFIT: %s", types.USD.FormatMoney(report.Profit))
	log.Infof("UNREALIZED PROFIT: %s", types.USD.FormatMoney(report.UnrealizedProfit))
}

func (report AverageCostPnlReport) SlackAttachment() slack.Attachment {
	var color = slackstyle.Red

	if report.UnrealizedProfit.Sign() > 0 {
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
			{Title: "Current Price", Value: report.Market.FormatPrice(report.LastPrice), Short: true},
			{Title: "Average Cost", Value: report.Market.FormatPrice(report.AverageCost), Short: true},

			// FIXME:
			// {Title: "Fee (USD)", Value: types.USD.FormatMoney(report.FeeInUSD), Short: true},
			{Title: "Stock", Value: report.Stock.String(), Short: true},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
