package bbgo

import (
	"fmt"
	"github.com/slack-go/slack"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

// Profit struct stores the PnL information
type Profit struct {
	Symbol string `json:"symbol"`

	// Profit is the profit of this trade made. negative profit means loss.
	Profit fixedpoint.Value `json:"profit" db:"profit"`

	// NetProfit is (profit - trading fee)
	NetProfit   fixedpoint.Value `json:"netProfit" db:"net_profit"`
	AverageCost fixedpoint.Value `json:"averageCost" db:"average_ost"`

	TradeAmount fixedpoint.Value `json:"tradeAmount" db:"trade_amount"`

	// ProfitMargin is a percentage of the profit and the capital amount
	ProfitMargin fixedpoint.Value `json:"profitMargin" db:"profit_margin"`

	// NetProfitMargin is a percentage of the net profit and the capital amount
	NetProfitMargin fixedpoint.Value `json:"netProfitMargin" db:"net_profit_margin"`

	QuoteCurrency string `json:"quoteCurrency" db:"quote_currency"`
	BaseCurrency  string `json:"baseCurrency" db:"base_currency"`

	// FeeInUSD is the summed fee of this profit,
	// you will need to convert the trade fee into USD since the fee currencies can be different.
	FeeInUSD           fixedpoint.Value `json:"feeInUSD" db:"fee_in_usd"`
	Time               time.Time        `json:"time" db:"time"`
	Strategy           string           `json:"strategy" db:"strategy"`
	StrategyInstanceID string           `json:"strategyInstanceID" db:"strategy_instance_id"`
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var color = pnlColor(p.Profit)
	var title = fmt.Sprintf("%s PnL ", p.Symbol)
	title += pnlEmojiMargin(p.Profit, p.ProfitMargin, defaultPnlLevelResolution) + " "
	title += pnlSignString(p.Profit) + " " + p.QuoteCurrency

	var fields []slack.AttachmentField

	if !p.NetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit",
			Value: pnlSignString(p.NetProfit) + " " + p.QuoteCurrency,
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

	if !p.TradeAmount.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Trade Amount",
			Value: p.TradeAmount.String() + " " + p.QuoteCurrency,
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
		emoji = pnlEmojiMargin(p.Profit, p.ProfitMargin, defaultPnlLevelResolution)
	} else {
		emoji = pnlEmojiSimple(p.Profit)
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

var lossEmoji = "🔥"
var profitEmoji = "💰"
var defaultPnlLevelResolution = fixedpoint.NewFromFloat(0.001)

func pnlColor(pnl fixedpoint.Value) string {
	if pnl.Sign() > 0 {
		return types.GreenColor
	}
	return types.RedColor
}

func pnlSignString(pnl fixedpoint.Value) string {
	if pnl.Sign() > 0 {
		return "+" + pnl.String()
	}
	return pnl.String()
}

func pnlEmojiSimple(pnl fixedpoint.Value) string {
	if pnl.Sign() < 0 {
		return lossEmoji
	}

	if pnl.IsZero() {
		return ""
	}

	return profitEmoji
}

func pnlEmojiMargin(pnl, margin, resolution fixedpoint.Value) (out string) {
	if margin.IsZero() {
		return pnlEmojiSimple(pnl)
	}

	if pnl.Sign() < 0 {
		out = lossEmoji
		level := (margin.Neg()).Div(resolution).Int()
		for i := 1; i < level; i++ {
			out += lossEmoji
		}
		return out
	}

	if pnl.IsZero() {
		return out
	}

	out = profitEmoji
	level := margin.Div(resolution).Int()
	for i := 1; i < level; i++ {
		out += profitEmoji
	}
	return out
}

type ProfitStats struct {
	Symbol        string `json:"symbol"`
	QuoteCurrency string `json:"quoteCurrency"`
	BaseCurrency  string `json:"baseCurrency"`

	AccumulatedPnL       fixedpoint.Value `json:"accumulatedPnL,omitempty"`
	AccumulatedNetProfit fixedpoint.Value `json:"accumulatedNetProfit,omitempty"`
	AccumulatedProfit    fixedpoint.Value `json:"accumulatedProfit,omitempty"`
	AccumulatedLoss      fixedpoint.Value `json:"accumulatedLoss,omitempty"`
	AccumulatedVolume    fixedpoint.Value `json:"accumulatedVolume,omitempty"`
	AccumulatedSince     int64            `json:"accumulatedSince,omitempty"`

	TodayPnL       fixedpoint.Value `json:"todayPnL,omitempty"`
	TodayNetProfit fixedpoint.Value `json:"todayNetProfit,omitempty"`
	TodayProfit    fixedpoint.Value `json:"todayProfit,omitempty"`
	TodayLoss      fixedpoint.Value `json:"todayLoss,omitempty"`
	TodaySince     int64            `json:"todaySince,omitempty"`
}

func (s *ProfitStats) Init(market types.Market) {
	s.Symbol = market.Symbol
	s.BaseCurrency = market.BaseCurrency
	s.QuoteCurrency = market.QuoteCurrency
	if s.AccumulatedSince == 0 {
		s.AccumulatedSince = time.Now().Unix()
	}
}

func (s *ProfitStats) AddProfit(profit Profit) {
	s.AccumulatedPnL = s.AccumulatedPnL.Add(profit.Profit)
	s.AccumulatedNetProfit = s.AccumulatedNetProfit.Add(profit.NetProfit)
	s.TodayPnL = s.TodayPnL.Add(profit.Profit)
	s.TodayNetProfit = s.TodayNetProfit.Add(profit.NetProfit)

	if profit.Profit.Sign() < 0 {
		s.AccumulatedLoss = s.AccumulatedLoss.Add(profit.Profit)
		s.TodayLoss = s.TodayLoss.Add(profit.Profit)
	} else if profit.Profit.Sign() > 0 {
		s.AccumulatedProfit = s.AccumulatedLoss.Add(profit.Profit)
		s.TodayProfit = s.TodayProfit.Add(profit.Profit)
	}
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.IsOver24Hours() {
		s.ResetToday()
	}

	s.AccumulatedVolume = s.AccumulatedVolume.Add(trade.Quantity)
}

func (s *ProfitStats) IsOver24Hours() bool {
	return time.Since(time.Unix(s.TodaySince, 0)) > 24*time.Hour
}

func (s *ProfitStats) ResetToday() {
	s.TodayPnL = fixedpoint.Zero
	s.TodayNetProfit = fixedpoint.Zero
	s.TodayProfit = fixedpoint.Zero
	s.TodayLoss = fixedpoint.Zero

	var beginningOfTheDay = util.BeginningOfTheDay(time.Now().Local())
	s.TodaySince = beginningOfTheDay.Unix()
}

func (s *ProfitStats) PlainText() string {
	since := time.Unix(s.AccumulatedSince, 0).Local()
	return fmt.Sprintf("%s Profit Today\n"+
		"Profit %s %s\n"+
		"Net profit %s %s\n"+
		"Trade Loss %s %s\n"+
		"Summary:\n"+
		"Accumulated Profit %s %s\n"+
		"Accumulated Net Profit %s %s\n"+
		"Accumulated Trade Loss %s %s\n"+
		"Since %s",
		s.Symbol,
		s.TodayPnL.String(), s.QuoteCurrency,
		s.TodayNetProfit.String(), s.QuoteCurrency,
		s.TodayLoss.String(), s.QuoteCurrency,
		s.AccumulatedPnL.String(), s.QuoteCurrency,
		s.AccumulatedNetProfit.String(), s.QuoteCurrency,
		s.AccumulatedLoss.String(), s.QuoteCurrency,
		since.Format(time.RFC822),
	)
}

func (s *ProfitStats) SlackAttachment() slack.Attachment {
	var color = pnlColor(s.AccumulatedPnL)
	var title = fmt.Sprintf("%s Accumulated PnL %s %s", s.Symbol, pnlSignString(s.AccumulatedPnL), s.QuoteCurrency)

	since := time.Unix(s.AccumulatedSince, 0).Local()
	title += " Since " + since.Format(time.RFC822)

	var fields []slack.AttachmentField

	if !s.TodayPnL.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "P&L Today",
			Value: pnlSignString(s.TodayPnL) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit Today",
			Value: pnlSignString(s.TodayProfit) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayNetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit Today",
			Value: pnlSignString(s.TodayNetProfit) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.TodayLoss.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Loss Today",
			Value: pnlSignString(s.TodayLoss) + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if !s.AccumulatedPnL.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated P&L",
			Value: pnlSignString(s.AccumulatedPnL) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Profit",
			Value: pnlSignString(s.AccumulatedProfit) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedNetProfit.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Net Profit",
			Value: pnlSignString(s.AccumulatedNetProfit) + " " + s.QuoteCurrency,
		})
	}

	if !s.AccumulatedLoss.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Loss",
			Value: pnlSignString(s.AccumulatedLoss) + " " + s.QuoteCurrency,
		})
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Fields: fields,
		// Footer:        "",
	}
}
