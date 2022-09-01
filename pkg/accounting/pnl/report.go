package pnl

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/types"
)

type AverageCostPnLReport struct {
	LastPrice fixedpoint.Value `json:"lastPrice"`
	StartTime time.Time        `json:"startTime"`
	Symbol    string           `json:"symbol"`
	Market    types.Market     `json:"market"`

	NumTrades        int              `json:"numTrades"`
	Profit           fixedpoint.Value `json:"profit"`
	UnrealizedProfit fixedpoint.Value `json:"unrealizedProfit"`

	NetProfit   fixedpoint.Value `json:"netProfit"`
	GrossProfit fixedpoint.Value `json:"grossProfit"`
	GrossLoss   fixedpoint.Value `json:"grossLoss"`
	Position    *types.Position  `json:"position,omitempty"`

	AverageCost       fixedpoint.Value            `json:"averageCost"`
	BuyVolume         fixedpoint.Value            `json:"buyVolume,omitempty"`
	SellVolume        fixedpoint.Value            `json:"sellVolume,omitempty"`
	FeeInUSD          fixedpoint.Value            `json:"feeInUSD"`
	BaseAssetPosition fixedpoint.Value            `json:"baseAssetPosition"`
	CurrencyFees      map[string]fixedpoint.Value `json:"currencyFees"`
}

func (report *AverageCostPnLReport) JSON() ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func (report AverageCostPnLReport) Print() {
	color.Green("TRADES SINCE: %v", report.StartTime)
	color.Green("NUMBER OF TRADES: %d", report.NumTrades)
	color.Green(report.Position.String())
	color.Green("AVERAGE COST: %s", types.USD.FormatMoney(report.AverageCost))
	color.Green("BASE ASSET POSITION: %s", report.BaseAssetPosition.String())

	color.Green("TOTAL BUY VOLUME: %v", report.BuyVolume)
	color.Green("TOTAL SELL VOLUME: %v", report.SellVolume)

	color.Green("CURRENT PRICE: %s", types.USD.FormatMoney(report.LastPrice))
	color.Green("CURRENCY FEES:")
	for currency, fee := range report.CurrencyFees {
		color.Green(" - %s: %s", currency, fee.String())
	}

	if report.Profit.Sign() > 0 {
		color.Green("PROFIT: %s", types.USD.FormatMoney(report.Profit))
	} else {
		color.Red("PROFIT: %s", types.USD.FormatMoney(report.Profit))
	}

	if report.UnrealizedProfit.Sign() > 0 {
		color.Green("UNREALIZED PROFIT: %s", types.USD.FormatMoney(report.UnrealizedProfit))
	} else {
		color.Red("UNREALIZED PROFIT: %s", types.USD.FormatMoney(report.UnrealizedProfit))
	}
}

func (report AverageCostPnLReport) SlackAttachment() slack.Attachment {
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
			{Title: "Base Asset Position", Value: report.BaseAssetPosition.String(), Short: true},
			{Title: "Number of Trades", Value: strconv.Itoa(report.NumTrades), Short: true},
		},
		Footer:     report.StartTime.Format(time.RFC822),
		FooterIcon: "",
	}
}
