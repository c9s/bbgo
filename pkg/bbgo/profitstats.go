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

	QuoteCurrency string `json:"quote_currency" db:"quote_currency"`
	BaseCurrency  string `json:"base_currency" db:"base_currency"`

	// FeeInUSD is the summed fee of this profit,
	// you will need to convert the trade fee into USD since the fee currencies can be different.
	FeeInUSD           fixedpoint.Value `json:"feeInUSD" db:"fee_in_usd"`
	Time               time.Time        `json:"time" db:"time"`
	Strategy           string           `json:"strategy" db:"strategy"`
	StrategyInstanceID string           `json:"strategyInstanceID" db:"strategy_instance_id"`
}

func (p *Profit) SlackAttachment() slack.Attachment {
	var title string = fmt.Sprintf("%s PnL ", p.Symbol)
	var color string

	if p.ProfitMargin != 0 {
		title += pnlEmojiMargin(p.Profit, p.ProfitMargin, defaultPnlLevelResolution) + " "
	} else {
		title += pnlEmojiSimple(p.Profit) + " "
	}

	if p.Profit > 0 {
		color = types.GreenColor
		title = "+" + p.Profit.String() + " " + p.QuoteCurrency
	} else {
		color = types.RedColor
		title = p.Profit.String() + " " + p.QuoteCurrency
	}

	var fields []slack.AttachmentField

	if p.NetProfit != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit",
			Value: p.NetProfit.String() + " " + p.QuoteCurrency,
			Short: true,
		})
	}

	if p.ProfitMargin != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit Margin",
			Value: p.ProfitMargin.Percentage(),
			Short: true,
		})
	}

	if p.NetProfitMargin != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit Margin",
			Value: p.NetProfitMargin.Percentage(),
			Short: true,
		})
	}

	if p.TradeAmount != 0.0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Trade Amount",
			Value: p.TradeAmount.String() + " " + p.QuoteCurrency,
			Short: true,
		})
	}

	if p.FeeInUSD != 0 {
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
	if p.ProfitMargin != 0 {
		emoji = pnlEmojiMargin(p.Profit, p.ProfitMargin, defaultPnlLevelResolution)
	} else {
		emoji = pnlEmojiSimple(p.Profit)
	}

	return fmt.Sprintf("%s trade profit %s %f %s (%.2f%%), net profit =~ %f %s (%.2f%%)",
		p.Symbol,
		emoji,
		p.Profit.Float64(), p.QuoteCurrency,
		p.ProfitMargin.Float64()*100.0,
		p.NetProfit.Float64(), p.QuoteCurrency,
		p.NetProfitMargin.Float64()*100.0,
	)
}

var lossEmoji = "🔥"
var profitEmoji = "💰"
var defaultPnlLevelResolution = fixedpoint.NewFromFloat(0.001)

func pnlEmojiSimple(pnl fixedpoint.Value) string {
	if pnl < 0 {
		return lossEmoji
	}

	if pnl == 0 {
		return ""
	}

	return profitEmoji
}

func pnlEmojiMargin(pnl, margin, resolution fixedpoint.Value) (out string) {
	if pnl < 0 {
		out = lossEmoji
		level := (-margin).Div(resolution).Floor()
		for i := 1; i < level.Int(); i++ {
			out += lossEmoji
		}
		return out
	}

	if pnl == 0 {
		return out
	}

	out = profitEmoji
	level := margin.Div(resolution).Floor()
	for i := 1; i < level.Int(); i++ {
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

func (s *ProfitStats) AddProfit(profit Profit) {
	s.AccumulatedPnL += profit.Profit
	s.AccumulatedNetProfit += profit.NetProfit
	s.TodayPnL += profit.Profit
	s.TodayNetProfit += profit.NetProfit

	if profit.Profit < 0 {
		s.AccumulatedLoss += profit.Profit
		s.TodayLoss += profit.Profit
	} else if profit.Profit > 0 {
		s.AccumulatedProfit += profit.Profit
		s.TodayProfit += profit.Profit
	}
}

func (s *ProfitStats) AddTrade(trade types.Trade) {
	if s.IsOver24Hours() {
		s.ResetToday()
	}

	s.AccumulatedVolume += fixedpoint.NewFromFloat(trade.Quantity)
}

func (s *ProfitStats) IsOver24Hours() bool {
	return time.Since(time.Unix(s.TodaySince, 0)) > 24*time.Hour
}

func (s *ProfitStats) ResetToday() {
	s.TodayPnL = 0
	s.TodayNetProfit = 0
	s.TodayProfit = 0
	s.TodayLoss = 0

	var beginningOfTheDay = util.BeginningOfTheDay(time.Now().Local())
	s.TodaySince = beginningOfTheDay.Unix()
}

func (s *ProfitStats) PlainText() string {
	since := time.Unix(s.AccumulatedSince, 0).Local()
	return fmt.Sprintf("today %s profit %f %s,\n"+
		"today %s net profit %f %s,\n"+
		"today %s trade loss %f %s\n"+
		"accumulated profit %f %s,\n"+
		"accumulated net profit %f %s,\n"+
		"accumulated trade loss %f %s\n"+
		"since %s",
		s.Symbol, s.TodayPnL.Float64(), s.QuoteCurrency,
		s.Symbol, s.TodayNetProfit.Float64(), s.QuoteCurrency,
		s.Symbol, s.TodayLoss.Float64(), s.QuoteCurrency,
		s.AccumulatedPnL.Float64(), s.QuoteCurrency,
		s.AccumulatedNetProfit.Float64(), s.QuoteCurrency,
		s.AccumulatedLoss.Float64(), s.QuoteCurrency,
		since.Format(time.RFC822),
	)
}

func (s *ProfitStats) SlackAttachment() slack.Attachment {
	var title string = fmt.Sprintf("%s Accumulated PnL ", s.Symbol)
	var color string
	if s.AccumulatedPnL > 0 {
		color = types.GreenColor
		title = "+" + s.AccumulatedPnL.String() + " " + s.QuoteCurrency
	} else {
		color = types.RedColor
		title = s.AccumulatedPnL.String() + " " + s.QuoteCurrency
	}

	since := time.Unix(s.AccumulatedSince, 0).Local()
	title += " Since " + since.Format(time.RFC822)

	var fields []slack.AttachmentField

	if s.TodayProfit != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Profit Today",
			Value: s.TodayProfit.String() + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if s.TodayNetProfit != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Net Profit Today",
			Value: s.TodayNetProfit.String() + " " + s.QuoteCurrency,
			Short: true,
		})
	}

	if s.TodayLoss != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Loss Today",
			Value: s.TodayLoss.String() + " " + s.QuoteCurrency,
			Short: true,
		})
	}


	if s.AccumulatedProfit != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Profit",
			Value: s.AccumulatedProfit.String() + " " + s.QuoteCurrency,
		})
	}

	if s.AccumulatedNetProfit != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Net Profit",
			Value: s.AccumulatedNetProfit.String() + " " + s.QuoteCurrency,
		})
	}

	if s.AccumulatedLoss != 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Accumulated Loss",
			Value: s.AccumulatedLoss.String() + " " + s.QuoteCurrency,
		})
	}

	return slack.Attachment{
		Color:  color,
		Title:  title,
		Fields: fields,
		// Footer:        "",
	}
}
