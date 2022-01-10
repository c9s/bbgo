package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util"
)

type PositionType string

const (
	PositionShort  = PositionType("Short")
	PositionLong   = PositionType("Long")
	PositionClosed = PositionType("Closed")
)

type ExchangeFee struct {
	MakerFeeRate fixedpoint.Value
	TakerFeeRate fixedpoint.Value
}

type PositionRisk struct {
	Leverage         fixedpoint.Value `json:"leverage"`
	LiquidationPrice fixedpoint.Value `json:"liquidationPrice"`
}

type Position struct {
	Symbol        string `json:"symbol"`
	BaseCurrency  string `json:"baseCurrency"`
	QuoteCurrency string `json:"quoteCurrency"`

	Market Market `json:"market"`

	Base        fixedpoint.Value `json:"base"`
	Quote       fixedpoint.Value `json:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost"`

	// ApproximateAverageCost adds the computed fee in quote in the average cost
	// This is used for calculating net profit
	ApproximateAverageCost fixedpoint.Value `json:"approximateAverageCost"`

	FeeRate          *ExchangeFee                 `json:"feeRate,omitempty"`
	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	// TotalFee stores the fee currency -> total fee quantity
	TotalFee map[string]fixedpoint.Value `json:"totalFee"`

	sync.Mutex
}

func (p *Position) GetBase() (base fixedpoint.Value) {
	p.Lock()
	base = p.Base
	p.Unlock()
	return base
}

type FuturesPosition struct {
	Symbol        string `json:"symbol"`
	BaseCurrency  string `json:"baseCurrency"`
	QuoteCurrency string `json:"quoteCurrency"`

	Market Market `json:"market"`

	Base        fixedpoint.Value `json:"base"`
	Quote       fixedpoint.Value `json:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost"`

	// ApproximateAverageCost adds the computed fee in quote in the average cost
	// This is used for calculating net profit
	ApproximateAverageCost fixedpoint.Value `json:"approximateAverageCost"`

	FeeRate          *ExchangeFee                 `json:"feeRate,omitempty"`
	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	// Futures data fields
	Isolated     bool  `json:"isolated"`
	UpdateTime   int64 `json:"updateTime"`
	PositionRisk *PositionRisk

	sync.Mutex
}

func NewPositionFromMarket(market Market) *Position {
	return &Position{
		Symbol:        market.Symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
		Market:        market,
		TotalFee:      make(map[string]fixedpoint.Value),
	}
}

func NewPosition(symbol, base, quote string) *Position {
	return &Position{
		Symbol:        symbol,
		BaseCurrency:  base,
		QuoteCurrency: quote,
		TotalFee:      make(map[string]fixedpoint.Value),
	}
}

func (p *Position) addTradeFee(trade Trade) {
	if p.TotalFee == nil {
		p.TotalFee = make(map[string]fixedpoint.Value)
	}
	p.TotalFee[trade.FeeCurrency] = p.TotalFee[trade.FeeCurrency] + fixedpoint.NewFromFloat(trade.Fee)
}

func (p *Position) Reset() {
	p.Base = 0
	p.Quote = 0
	p.AverageCost = 0
}

func (p *Position) SetFeeRate(exchangeFee ExchangeFee) {
	p.FeeRate = &exchangeFee
}

func (p *Position) SetExchangeFeeRate(ex ExchangeName, exchangeFee ExchangeFee) {
	if p.ExchangeFeeRates == nil {
		p.ExchangeFeeRates = make(map[ExchangeName]ExchangeFee)
	}

	p.ExchangeFeeRates[ex] = exchangeFee
}

func (p *Position) Type() PositionType {
	if p.Base > 0 {
		return PositionLong
	} else if p.Base < 0 {
		return PositionShort
	}
	return PositionClosed
}

func (p *Position) SlackAttachment() slack.Attachment {
	p.Lock()
	defer p.Unlock()

	averageCost := p.AverageCost
	base := p.Base
	quote := p.Quote

	var posType = p.Type()
	var color = ""

	if p.Base == 0 {
		color = "#cccccc"
	} else if p.Base > 0 {
		color = "#228B22"
	} else if p.Base < 0 {
		color = "#DC143C"
	}

	title := util.Render(string(posType)+` Position {{ .Symbol }} `, p)

	fields := []slack.AttachmentField{
		{Title: "Average Cost", Value: trimTrailingZeroFloat(averageCost.Float64()) + " " + p.QuoteCurrency, Short: true},
		{Title: p.BaseCurrency, Value: trimTrailingZeroFloat(base.Float64()), Short: true},
		{Title: p.QuoteCurrency, Value: trimTrailingZeroFloat(quote.Float64())},
	}

	if p.TotalFee != nil {
		for feeCurrency, fee := range p.TotalFee {
			if fee > 0 {
				fields = append(fields, slack.AttachmentField{
					Title: fmt.Sprintf("Fee (%s)", feeCurrency),
					Value: trimTrailingZeroFloat(fee.Float64()),
					Short: true,
				})
			}
		}
	}

	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title:  title,
		Color:  color,
		Fields: fields,
		Footer: util.Render("update time {{ . }}", time.Now().Format(time.RFC822)),
		// FooterIcon: "",
	}
}

func (p *Position) PlainText() (msg string) {
	posType := p.Type()
	msg = fmt.Sprintf("%s Position %s: average cost = %s, base = %s, quote = %s",
		posType,
		p.Symbol,
		trimTrailingZeroFloat(p.AverageCost.Float64()),
		trimTrailingZeroFloat(p.Base.Float64()),
		trimTrailingZeroFloat(p.Quote.Float64()),
	)

	if p.TotalFee != nil {
		for feeCurrency, fee := range p.TotalFee {
			msg += fmt.Sprintf("\nfee (%s) = %s", feeCurrency, trimTrailingZeroFloat(fee.Float64()))
		}
	}

	return msg
}

func (p *Position) String() string {
	return fmt.Sprintf("POSITION %s: average cost = %f, base = %f, quote = %f",
		p.Symbol,
		p.AverageCost.Float64(),
		p.Base.Float64(),
		p.Quote.Float64(),
	)
}

func (p *Position) BindStream(stream Stream) {
	stream.OnTradeUpdate(func(trade Trade) {
		if p.Symbol == trade.Symbol {
			p.AddTrade(trade)
		}
	})
}

func (p *Position) AddTrades(trades []Trade) (fixedpoint.Value, fixedpoint.Value, bool) {
	var totalProfitAmount, totalNetProfit fixedpoint.Value
	for _, trade := range trades {
		if profit, netProfit, madeProfit := p.AddTrade(trade); madeProfit {
			totalProfitAmount += profit
			totalNetProfit += netProfit
		}
	}

	return totalProfitAmount, totalNetProfit, totalProfitAmount != 0
}

func (p *Position) AddTrade(td Trade) (profit fixedpoint.Value, netProfit fixedpoint.Value, madeProfit bool) {
	price := fixedpoint.NewFromFloat(td.Price)
	quantity := fixedpoint.NewFromFloat(td.Quantity)
	quoteQuantity := fixedpoint.NewFromFloat(td.QuoteQuantity)
	fee := fixedpoint.NewFromFloat(td.Fee)

	// calculated fee in quote (some exchange accounts may enable platform currency fee discount, like BNB)
	var feeInQuote fixedpoint.Value = 0

	switch td.FeeCurrency {

	case p.BaseCurrency:
		quantity -= fee

	case p.QuoteCurrency:
		quoteQuantity -= fee

	default:
		if p.ExchangeFeeRates != nil {
			if exchangeFee, ok := p.ExchangeFeeRates[td.Exchange]; ok {
				if td.IsMaker {
					feeInQuote += exchangeFee.MakerFeeRate.Mul(quoteQuantity)
				} else {
					feeInQuote += exchangeFee.TakerFeeRate.Mul(quoteQuantity)
				}
			}
		} else if p.FeeRate != nil {
			if td.IsMaker {
				feeInQuote += p.FeeRate.MakerFeeRate.Mul(quoteQuantity)
			} else {
				feeInQuote += p.FeeRate.TakerFeeRate.Mul(quoteQuantity)
			}
		}
	}

	p.Lock()
	defer p.Unlock()

	p.addTradeFee(td)

	// Base > 0 means we're in long position
	// Base < 0  means we're in short position
	switch td.Side {

	case SideTypeBuy:
		if p.Base < 0 {
			// convert short position to long position
			if p.Base+quantity > 0 {
				profit = (p.AverageCost - price).Mul(-p.Base)
				netProfit = (p.ApproximateAverageCost - price).Mul(-p.Base) - feeInQuote
				p.Base += quantity
				p.Quote -= quoteQuantity
				p.AverageCost = price
				p.ApproximateAverageCost = price
				return profit, netProfit, true
			} else {
				// covering short position
				p.Base += quantity
				p.Quote -= quoteQuantity
				profit = (p.AverageCost - price).Mul(quantity)
				netProfit = (p.ApproximateAverageCost - price).Mul(quantity) - feeInQuote
				return profit, netProfit, true
			}
		}

		p.ApproximateAverageCost = (p.ApproximateAverageCost.Mul(p.Base) + quoteQuantity + feeInQuote).Div(p.Base + quantity)
		p.AverageCost = (p.AverageCost.Mul(p.Base) + quoteQuantity).Div(p.Base + quantity)
		p.Base += quantity
		p.Quote -= quoteQuantity

		return 0, 0, false

	case SideTypeSell:
		if p.Base > 0 {
			// convert long position to short position
			if p.Base-quantity < 0 {
				profit = (price - p.AverageCost).Mul(p.Base)
				netProfit = (price - p.ApproximateAverageCost).Mul(p.Base) - feeInQuote
				p.Base -= quantity
				p.Quote += quoteQuantity
				p.AverageCost = price
				p.ApproximateAverageCost = price
				return profit, netProfit, true
			} else {
				p.Base -= quantity
				p.Quote += quoteQuantity
				profit = (price - p.AverageCost).Mul(quantity)
				netProfit = (price - p.ApproximateAverageCost).Mul(quantity) - feeInQuote
				return profit, netProfit, true
			}
		}

		// handling short position, since Base here is negative we need to reverse the sign
		p.ApproximateAverageCost = (p.ApproximateAverageCost.Mul(-p.Base) + quoteQuantity - feeInQuote).Div(-p.Base + quantity)
		p.AverageCost = (p.AverageCost.Mul(-p.Base) + quoteQuantity).Div(-p.Base + quantity)
		p.Base -= quantity
		p.Quote += quoteQuantity

		return 0, 0, false
	}

	return 0, 0, false
}
