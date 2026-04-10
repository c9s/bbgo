package grid2

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
)

var _ common.TabularStats = (*GridProfitStats)(nil)

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
	Since            *time.Time                  `json:"since,omitempty"`
	InitialOrderID   uint64                      `json:"initialOrderID"`

	DailyFee            map[time.Time]map[string]fixedpoint.Value `json:"dailyFee,omitempty"`
	DailyNumOfArbitrage map[time.Time]int                         `json:"dailyArbitrageCount,omitempty"`
	DailyProfit         map[time.Time]fixedpoint.Value            `json:"dailyProfit,omitempty"`

	// ttl is the ttl to keep in persistence
	ttl time.Duration
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

		DailyFee:            make(map[time.Time]map[string]fixedpoint.Value),
		DailyNumOfArbitrage: make(map[time.Time]int),
		DailyProfit:         make(map[time.Time]fixedpoint.Value),
	}
}

func (s *GridProfitStats) SetTTL(ttl time.Duration) {
	if ttl.Nanoseconds() <= 0 {
		return
	}
	s.ttl = ttl
}

func (s *GridProfitStats) Expiration() time.Duration {
	return s.ttl
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

	// accumulate daily fee
	dateKey := getDateKey(trade.Time.Time())
	if s.DailyFee == nil {
		s.DailyFee = make(map[time.Time]map[string]fixedpoint.Value)
	}
	if s.DailyFee[dateKey] == nil {
		s.DailyFee[dateKey] = make(map[string]fixedpoint.Value)
	}
	s.DailyFee[dateKey][trade.FeeCurrency] = s.DailyFee[dateKey][trade.FeeCurrency].Add(trade.Fee)
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

	// Normalize to midnight UTC for daily aggregation
	dateKey := getDateKey(profit.Time)
	if s.DailyNumOfArbitrage == nil {
		s.DailyNumOfArbitrage = make(map[time.Time]int)
	}
	s.DailyNumOfArbitrage[dateKey]++

	if s.DailyProfit == nil {
		s.DailyProfit = make(map[time.Time]fixedpoint.Value)
	}
	s.DailyProfit[dateKey] = s.DailyProfit[dateKey].Add(profit.Profit)
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

// Implements common.TabularStats interface
func (s *GridProfitStats) SummaryHeader() []string {
	feeCurrencies := s.collectFeeCurrencies()
	header := []string{"date", "arbitrage_count", "profit"}
	for _, currency := range feeCurrencies {
		header = append(header, "fee_"+currency)
	}
	return header
}

func (s *GridProfitStats) SummaryRecords() [][]string {
	feeCurrencies := s.collectFeeCurrencies()

	// collect and sort date keys
	dateKeys := make(map[time.Time]struct{})
	for k := range s.DailyNumOfArbitrage {
		dateKeys[k] = struct{}{}
	}
	for k := range s.DailyProfit {
		dateKeys[k] = struct{}{}
	}
	for k := range s.DailyFee {
		dateKeys[k] = struct{}{}
	}

	dates := make([]time.Time, 0, len(dateKeys))
	for k := range dateKeys {
		dates = append(dates, k)
	}
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	var records [][]string
	for _, date := range dates {
		count := s.DailyNumOfArbitrage[date]
		profit := s.DailyProfit[date]
		dateStr := date.Format(time.DateOnly)
		record := []string{dateStr, strconv.Itoa(count), profit.String()}
		dailyFees := s.DailyFee[date]
		for _, currency := range feeCurrencies {
			if dailyFees != nil {
				record = append(record, dailyFees[currency].String())
			} else {
				record = append(record, fixedpoint.Zero.String())
			}
		}
		records = append(records, record)
	}
	return records
}

// collectFeeCurrencies returns a sorted list of unique fee currencies across all days.
func (s *GridProfitStats) collectFeeCurrencies() []string {
	currencySet := make(map[string]struct{})
	for _, fees := range s.DailyFee {
		for currency := range fees {
			currencySet[currency] = struct{}{}
		}
	}
	currencies := make([]string, 0, len(currencySet))
	for c := range currencySet {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	return currencies
}

func getDateKey(ts time.Time) time.Time {
	dateKey := time.Date(
		ts.Year(), ts.Month(), ts.Day(),
		0, 0, 0, 0,
		ts.Location(),
	).UTC()
	return dateKey
}
