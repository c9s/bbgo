package types

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
)

// Profit struct stores the PnL information
type Profit struct {
	// --- position related fields
	// -------------------------------------------
	// Symbol is the symbol of the position
	Symbol        string           `json:"symbol"`
	QuoteCurrency string           `json:"quoteCurrency" db:"quote_currency"`
	BaseCurrency  string           `json:"baseCurrency" db:"base_currency"`
	AverageCost   fixedpoint.Value `json:"averageCost" db:"average_cost"`

	// profit related fields
	// -------------------------------------------
	// Profit is the profit of this trade made. negative profit means loss.
	Profit fixedpoint.Value `json:"profit" db:"profit"`

	// NetProfit is (profit - trading fee)
	NetProfit fixedpoint.Value `json:"netProfit" db:"net_profit"`

	// ProfitMargin is a percentage of the profit and the capital amount
	ProfitMargin fixedpoint.Value `json:"profitMargin" db:"profit_margin"`

	// NetProfitMargin is a percentage of the net profit and the capital amount
	NetProfitMargin fixedpoint.Value `json:"netProfitMargin" db:"net_profit_margin"`

	// trade related fields
	// --------------------------------------------
	// TradeID is the exchange trade id of that trade
	Trade         *Trade           `json:"trade,omitempty" db:"-"`
	TradeID       uint64           `json:"tradeID" db:"trade_id"`
	OrderID       uint64           `json:"orderID,omitempty"`
	Side          SideType         `json:"side" db:"side"`
	IsBuyer       bool             `json:"isBuyer" db:"is_buyer"`
	IsMaker       bool             `json:"isMaker" db:"is_maker"`
	Price         fixedpoint.Value `json:"price" db:"price"`
	Quantity      fixedpoint.Value `json:"quantity" db:"quantity"`
	QuoteQuantity fixedpoint.Value `json:"quoteQuantity" db:"quote_quantity"`

	// FeeInUSD is the summed fee of this profit,
	// you will need to convert the trade fee into USD since the fee currencies can be different.
	FeeInUSD    fixedpoint.Value `json:"feeInUSD" db:"fee_in_usd"`
	Fee         fixedpoint.Value `json:"fee" db:"fee"`
	FeeCurrency string           `json:"feeCurrency" db:"fee_currency"`
	Exchange    ExchangeName     `json:"exchange" db:"exchange"`
	IsMargin    bool             `json:"isMargin" db:"is_margin"`
	IsFutures   bool             `json:"isFutures" db:"is_futures"`
	IsIsolated  bool             `json:"isIsolated" db:"is_isolated"`
	TradedAt    time.Time        `json:"tradedAt" db:"traded_at"`

	PositionOpenedAt time.Time `json:"positionOpenedAt" db:"-"`

	// strategy related fields
	Strategy           string `json:"strategy" db:"strategy"`
	StrategyInstanceID string `json:"strategyInstanceID" db:"strategy_instance_id"`
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var color = style.PnLColor(p.Profit)
	var title = fmt.Sprintf("%s PnL ", p.Symbol)
	title += style.PnLEmojiMargin(p.Profit, p.ProfitMargin, style.DefaultPnLLevelResolution) + " "
	title += style.PnLSignString(p.Profit) + " " + p.QuoteCurrency

	var fields []slack.AttachmentField

	if !p.NetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit",
			Value: style.PnLSignString(p.NetProfit) + " " + p.QuoteCurrency,
			Short: true,
		})
	}

	if !p.ProfitMargin.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit Margin",
			Value: p.ProfitMargin.Percentage(),
			Short: true,
		})
	}

	if !p.NetProfitMargin.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit Margin",
			Value: p.NetProfitMargin.Percentage(),
			Short: true,
		})
	}

	if !p.QuoteQuantity.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Trade Amount",
			Value: p.QuoteQuantity.String() + " " + p.QuoteCurrency,
			Short: true,
		})
	}

	if !p.FeeInUSD.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Fee In USD",
			Value: p.FeeInUSD.String() + " USD",
			Short: true,
		})
	}

	if len(p.Strategy) != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Strategy",
			Value: p.Strategy,
			Short: true,
		})
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Fields: fields,
		// Footer:        "",
	}
}

func (p *Profit) PlainText() string {
	var emoji string
	if !p.ProfitMargin.IsZero() {
		emoji = style.PnLEmojiMargin(p.Profit, p.ProfitMargin, style.DefaultPnLLevelResolution)
	} else {
		emoji = style.PnLEmojiSimple(p.Profit)
	}

	return fmt.Sprintf("%s trade profit %s %s %s (%s), net profit =~ %s %s (%s)",
		p.Symbol,
		emoji,
		p.Profit.String(), p.QuoteCurrency,
		p.ProfitMargin.Percentage(),
		p.NetProfit.String(), p.QuoteCurrency,
		p.NetProfitMargin.Percentage(),
	)
}

type ProfitStats struct {
	Symbol        string `json:"symbol"`
	QuoteCurrency string `json:"quoteCurrency"`
	BaseCurrency  string `json:"baseCurrency"`

	AccumulatedPnL         fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedNetProfit   fixedpoint.Value `json:"accumulatedNetProfit,omitempty"`
	AccumulatedGrossProfit fixedpoint.Value `json:"accumulatedGrossProfit,omitempty"`
	AccumulatedGrossLoss   fixedpoint.Value `json:"accumulatedGrossLoss,omitempty"`
	AccumulatedVolume      fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedSince       int64            `json:"accumulatedSince,omitempty"`

	TodayPnL         fixedpoint.Value `json:"todayPnL,omitempty"`
	TodayNetProfit   fixedpoint.Value `json:"todayNetProfit,omitempty"`
	TodayGrossProfit fixedpoint.Value `json:"todayGrossProfit,omitempty"`
	TodayGrossLoss   fixedpoint.Value `json:"todayGrossLoss,omitempty"`
	TodaySince       int64            `json:"todaySince,omitempty"`
}

func NewProfitStats(market Market) *ProfitStats {
	return &ProfitStats{
		Symbol:                 market.Symbol,
		QuoteCurrency:          market.QuoteCurrency,
		BaseCurrency:           market.BaseCurrency,
		AccumulatedPnL:         fixedpoint.Zero,
		AccumulatedNetProfit:   fixedpoint.Zero,
		AccumulatedGrossProfit: fixedpoint.Zero,
		AccumulatedGrossLoss:   fixedpoint.Zero,
		AccumulatedVolume:      fixedpoint.Zero,
		AccumulatedSince:       0,
		TodayPnL:               fixedpoint.Zero,
		TodayNetProfit:         fixedpoint.Zero,
		TodayGrossProfit:       fixedpoint.Zero,
		TodayGrossLoss:         fixedpoint.Zero,
		TodaySince:             0,
	}
}

// Init
// Deprecated: use NewProfitStats instead
func (s *ProfitStats) Init(market Market) {
	s.Symbol = market.Symbol
	s.BaseCurrency = market.BaseCurrency
	s.QuoteCurrency = market.QuoteCurrency
	if s.AccumulatedSince == 0 {
		s.AccumulatedSince = time.Now().Unix()
	}
}

func (s *ProfitStats) AddProfit(profit Profit) {
	if s.IsOver24Hours() {
		s.ResetToday(profit.TradedAt)
	}

	// since field guard
	if s.AccumulatedSince == 0 {
		s.AccumulatedSince = profit.TradedAt.Unix()
	}

	if s.TodaySince == 0 {
		var beginningOfTheDay = BeginningOfTheDay(profit.TradedAt.Local())
		s.TodaySince = beginningOfTheDay.Unix()
	}

	s.AccumulatedPnL = s.AccumulatedPnL.Add(profit.Profit)
	s.AccumulatedNetProfit = s.AccumulatedNetProfit.Add(profit.NetProfit)
	s.TodayPnL = s.TodayPnL.Add(profit.Profit)
	s.TodayNetProfit = s.TodayNetProfit.Add(profit.NetProfit)

	if profit.Profit.Sign() > 0 {
		s.AccumulatedGrossProfit = s.AccumulatedGrossProfit.Add(profit.Profit)
		s.TodayGrossProfit = s.TodayGrossProfit.Add(profit.Profit)
	} else if profit.Profit.Sign() < 0 {
		s.AccumulatedGrossLoss = s.AccumulatedGrossLoss.Add(profit.Profit)
		s.TodayGrossLoss = s.TodayGrossLoss.Add(profit.Profit)
	}
}

func (s *ProfitStats) AddTrade(trade Trade) {
	if s.IsOver24Hours() {
		s.ResetToday(trade.Time.Time())
	}

	s.AccumulatedVolume = s.AccumulatedVolume.Add(trade.Quantity)
}

// IsOver24Hours checks if the since time is over 24 hours
func (s *ProfitStats) IsOver24Hours() bool {
	return time.Since(time.Unix(s.TodaySince, 0)) >= 24*time.Hour
}

func (s *ProfitStats) ResetToday(t time.Time) {
	s.TodayPnL = fixedpoint.Zero
	s.TodayNetProfit = fixedpoint.Zero
	s.TodayGrossProfit = fixedpoint.Zero
	s.TodayGrossLoss = fixedpoint.Zero

	var beginningOfTheDay = BeginningOfTheDay(t.Local())
	s.TodaySince = beginningOfTheDay.Unix()
}

func (s *ProfitStats) PlainText() string {
	since := time.Unix(s.AccumulatedSince, 0).Local()
	return fmt.Sprintf("%s Profit Today\n"+
		"Profit %s %s\n"+
		"Net profit %s %s\n"+
		"Gross Loss %s %s\n"+
		"Summary:\n"+
		"Accumulated Profit %s %s\n"+
		"Accumulated Net Profit %s %s\n"+
		"Accumulated Gross Loss %s %s\n"+
		"Since %s",
		s.Symbol,
		s.TodayPnL.String(), s.QuoteCurrency,
		s.TodayNetProfit.String(), s.QuoteCurrency,
		s.TodayGrossLoss.String(), s.QuoteCurrency,
		s.AccumulatedPnL.String(), s.QuoteCurrency,
		s.AccumulatedNetProfit.String(), s.QuoteCurrency,
		s.AccumulatedGrossLoss.String(), s.QuoteCurrency,
		since.Format(time.RFC822),
	)
}

func (s *ProfitStats) SlackAttachment() slack.Attachment {
	var color = style.PnLColor(s.AccumulatedPnL)
	var title = fmt.Sprintf("%s Accumulated PnL %s %s", s.Symbol, style.PnLSignString(s.AccumulatedPnL), s.QuoteCurrency)

	since := time.Unix(s.AccumulatedSince, 0).Local()
	title += " Since " + since.Format(time.RFC822)

	var fields []slack.AttachmentField

	if !s.TodayPnL.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "P&L Today",
			Value: style.PnLSignString(s.TodayPnL) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayNetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit Today",
			Value: style.PnLSignString(s.TodayNetProfit) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayGrossProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Gross Profit Today",
			Value: style.PnLSignString(s.TodayGrossProfit) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayGrossLoss.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Gross Loss Today",
			Value: style.PnLSignString(s.TodayGrossLoss) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.AccumulatedPnL.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated P&L",
			Value: style.PnLSignString(s.AccumulatedPnL) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedGrossProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Gross Profit",
			Value: style.PnLSignString(s.AccumulatedGrossProfit) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedGrossLoss.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Gross Loss",
			Value: style.PnLSignString(s.AccumulatedGrossLoss) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedNetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Net Profit",
			Value: style.PnLSignString(s.AccumulatedNetProfit) + " " + s.QuoteCurrency,
		})
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Fields: fields,
		// Footer:        "",
	}
}
