package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/slack-go/slack"
)

type ExchangeFee struct {
	MakerFeeRate fixedpoint.Value
	TakerFeeRate fixedpoint.Value
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

	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	Isolated               bool             		  `json:"isolated"`
	Leverage               fixedpoint.Value           `json:"leverage"`
	InitialMargin          fixedpoint.Value           `json:"initialMargin"`
	MaintMargin            fixedpoint.Value           `json:"maintMargin"`
	OpenOrderInitialMargin fixedpoint.Value           `json:"openOrderInitialMargin"`
	PositionInitialMargin  fixedpoint.Value           `json:"positionInitialMargin"`
	UnrealizedProfit       fixedpoint.Value           `json:"unrealizedProfit"`
	EntryPrice             fixedpoint.Value           `json:"entryPrice"`
	MaxNotional            fixedpoint.Value           `json:"maxNotional"`
	PositionSide           string 					  `json:"positionSide"`
	PositionAmt            fixedpoint.Value           `json:"positionAmt"`
	Notional               fixedpoint.Value           `json:"notional"`
	IsolatedWallet         fixedpoint.Value           `json:"isolatedWallet"`
	UpdateTime             int64           			   `json:"updateTime"`

	sync.Mutex
}

func NewPositionFromMarket(market Market) *Position {
	return &Position{
		Symbol:        market.Symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
		Market:        market,
	}
}

func NewPosition(symbol, base, quote string) *Position {
	return &Position{
		Symbol:        symbol,
		BaseCurrency:  base,
		QuoteCurrency: quote,
	}
}

func (p *Position) Reset() {
	p.Base = 0
	p.Quote = 0
	p.AverageCost = 0
}

func (p *Position) SetExchangeFeeRate(ex ExchangeName, exchangeFee ExchangeFee) {
	if p.ExchangeFeeRates == nil {
		p.ExchangeFeeRates = make(map[ExchangeName]ExchangeFee)
	}

	p.ExchangeFeeRates[ex] = exchangeFee
}

func (p *Position) SlackAttachment() slack.Attachment {
	p.Lock()
	averageCost := p.AverageCost
	base := p.Base
	quote := p.Quote
	p.Unlock()

	var posType = ""
	var color = ""

	if p.Base == 0 {
		color = "#cccccc"
		posType = "Closed"
	} else if p.Base > 0 {
		posType = "Long"
		color = "#228B22"
	} else if p.Base < 0 {
		posType = "Short"
		color = "#DC143C"
	}

	title := util.Render(posType+` Position {{ .Symbol }} `, p)
	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title: title,
		Color: color,
		Fields: []slack.AttachmentField{
			{Title: "Average Cost", Value: util.FormatFloat(averageCost.Float64(), 2), Short: true},
			{Title: p.BaseCurrency, Value: util.FormatFloat(base.Float64(), 4), Short: true},
			{Title: p.QuoteCurrency, Value: util.FormatFloat(quote.Float64(), 2)},
		},
		Footer: util.Render("update time {{ . }}", time.Now().Format(time.RFC822)),
		// FooterIcon: "",
	}
}

func (p *Position) PlainText() string {
	return fmt.Sprintf("Position %s: average cost = %f, base = %f, quote = %f",
		p.Symbol,
		p.AverageCost.Float64(),
		p.Base.Float64(),
		p.Quote.Float64(),
	)
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

func (p *Position) AddTrade(t Trade) (profit fixedpoint.Value, netProfit fixedpoint.Value, madeProfit bool) {
	price := fixedpoint.NewFromFloat(t.Price)
	quantity := fixedpoint.NewFromFloat(t.Quantity)
	quoteQuantity := fixedpoint.NewFromFloat(t.QuoteQuantity)
	fee := fixedpoint.NewFromFloat(t.Fee)

	// calculated fee in quote (some exchange accounts may enable platform currency fee discount, like BNB)
	var feeInQuote fixedpoint.Value = 0

	switch t.FeeCurrency {

	case p.BaseCurrency:
		quantity -= fee

	case p.QuoteCurrency:
		quoteQuantity -= fee

	default:
		if p.ExchangeFeeRates != nil {
			if exchangeFee, ok := p.ExchangeFeeRates[t.Exchange]; ok {
				if t.IsMaker {
					feeInQuote += exchangeFee.MakerFeeRate.Mul(quoteQuantity)
				} else {
					feeInQuote += exchangeFee.TakerFeeRate.Mul(quoteQuantity)
				}
			}
		}
	}

	p.Lock()
	defer p.Unlock()

	// Base > 0 means we're in long position
	// Base < 0  means we're in short position
	switch t.Side {

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
