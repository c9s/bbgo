package grid2

import (
	"fmt"
	"strconv"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
)

type GridProfitStats struct {
	Symbol           string                      `json:"symbol"`
	TotalBaseProfit  fixedpoint.Value            `json:"totalBaseProfit,omitempty"`
	TotalQuoteProfit fixedpoint.Value            `json:"totalQuoteProfit,omitempty"`
	FloatProfit      fixedpoint.Value            `json:"floatProfit,omitempty"`
	GridProfit       fixedpoint.Value            `json:"gridProfit,omitempty"`
	ArbitrageCount   int                         `json:"arbitrageCount,omitempty"`
	TotalFee         map[string]fixedpoint.Value `json:"totalFee,omitempty"`
	Volume           fixedpoint.Value            `json:"volume,omitempty"`
	Market           types.Market                `json:"market,omitempty"`
	ProfitEntries    []*GridProfit               `json:"profitEntries,omitempty"`
	Since            *time.Time                  `json:"since,omitempty"`
	InitialOrderID   uint64                      `json:"initialOrderID"`
}

func newGridProfitStats(market types.Market) *GridProfitStats {
	return &GridProfitStats{
		Symbol:           market.Symbol,
		TotalBaseProfit:  fixedpoint.Zero,
		TotalQuoteProfit: fixedpoint.Zero,
		FloatProfit:      fixedpoint.Zero,
		GridProfit:       fixedpoint.Zero,
		ArbitrageCount:   0,
		TotalFee:         make(map[string]fixedpoint.Value),
		Volume:           fixedpoint.Zero,
		Market:           market,
		ProfitEntries:    nil,
	}
}

func (s *GridProfitStats) AddTrade(trade types.Trade) {
	if s.TotalFee == nil {
		s.TotalFee = make(map[string]fixedpoint.Value)
	}

	if fee, ok := s.TotalFee[trade.FeeCurrency]; ok {
		s.TotalFee[trade.FeeCurrency] = fee.Add(trade.Fee)
	} else {
		s.TotalFee[trade.FeeCurrency] = trade.Fee
	}

	if s.Since == nil {
		t := trade.Time.Time()
		s.Since = &t
	}
}

func (s *GridProfitStats) AddProfit(profit *GridProfit) {
	// increase arbitrage count per profit round
	s.ArbitrageCount++

	switch profit.Currency {
	case s.Market.QuoteCurrency:
		s.TotalQuoteProfit = s.TotalQuoteProfit.Add(profit.Profit)
	case s.Market.BaseCurrency:
		s.TotalBaseProfit = s.TotalBaseProfit.Add(profit.Profit)
	}

	s.ProfitEntries = append(s.ProfitEntries, profit)
}

func (s *GridProfitStats) SlackAttachment() slack.Attachment {
	var fields = []slack.AttachmentField{
		{
			Title: "Arbitrage Count",
			Value: strconv.Itoa(s.ArbitrageCount),
			Short: true,
		},
	}

	if !s.FloatProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Float Profit",
			Value: style.PnLSignString(s.FloatProfit),
			Short: true,
		})
	}

	if !s.GridProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Total Grid Profit",
			Value: style.PnLSignString(s.GridProfit),
			Short: true,
		})
	}

	if !s.TotalQuoteProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Total Quote Profit",
			Value: style.PnLSignString(s.TotalQuoteProfit),
			Short: true,
		})
	}

	if !s.TotalBaseProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Total Base Profit",
			Value: style.PnLSignString(s.TotalBaseProfit),
			Short: true,
		})
	}

	if len(s.TotalFee) > 0 {
		for feeCurrency, fee := range s.TotalFee {
			fields = append(fields, slack.AttachmentField{
				Title: fmt.Sprintf("Fee (%s)", feeCurrency),
				Value: fee.String() + " " + feeCurrency,
				Short: true,
			})
		}
	}

	footer := "Total grid profit stats"
	if s.Since != nil {
		footer += fmt.Sprintf(" since %s", s.Since.String())
	}

	title := fmt.Sprintf("%s Grid Profit Stats", s.Symbol)
	return slack.Attachment{
		Title:  title,
		Color:  "warning",
		Fields: fields,
		Footer: footer,
	}
}

func (s *GridProfitStats) String() string {
	return s.PlainText()
}

func (s *GridProfitStats) PlainText() string {
	var o string

	o = fmt.Sprintf("%s Grid Profit Stats", s.Symbol)

	o += fmt.Sprintf(" Arbitrage count: %d", s.ArbitrageCount)

	if !s.FloatProfit.IsZero() {
		o += " Float profit: " + style.PnLSignString(s.FloatProfit)
	}

	if !s.GridProfit.IsZero() {
		o += " Grid profit: " + style.PnLSignString(s.GridProfit)
	}

	if !s.TotalQuoteProfit.IsZero() {
		o += " Total quote profit: " + style.PnLSignString(s.TotalQuoteProfit) + " " + s.Market.QuoteCurrency
	}

	if !s.TotalBaseProfit.IsZero() {
		o += " Total base profit: " + style.PnLSignString(s.TotalBaseProfit) + " " + s.Market.BaseCurrency
	}

	if len(s.TotalFee) > 0 {
		for feeCurrency, fee := range s.TotalFee {
			o += fmt.Sprintf(" Fee (%s)", feeCurrency) + fee.String() + " " + feeCurrency
		}
	}

	if s.Since != nil {
		o += fmt.Sprintf(" Since %s", s.Since.String())
	}

	return o
}
