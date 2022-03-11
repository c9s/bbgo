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
	Symbol        string `json:"symbol" db:"symbol"`
	BaseCurrency  string `json:"baseCurrency" db:"base"`
	QuoteCurrency string `json:"quoteCurrency" db:"quote"`

	Market Market `json:"market,omitempty"`

	Base        fixedpoint.Value `json:"base" db:"base"`
	Quote       fixedpoint.Value `json:"quote" db:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost" db:"average_cost"`

	// ApproximateAverageCost adds the computed fee in quote in the average cost
	// This is used for calculating net profit
	ApproximateAverageCost fixedpoint.Value `json:"approximateAverageCost"`

	FeeRate          *ExchangeFee                 `json:"feeRate,omitempty"`
	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	// TotalFee stores the fee currency -> total fee quantity
	TotalFee map[string]fixedpoint.Value `json:"totalFee" db:"-"`

	ChangedAt time.Time `json:"changedAt,omitempty" db:"changed_at"`

	Strategy           string `json:"strategy,omitempty" db:"strategy"`
	StrategyInstanceID string `json:"strategyInstanceID,omitempty" db:"strategy_instance_id"`

	AccumulatedProfit fixedpoint.Value `json:"accumulatedProfit,omitempty" db:"accumulated_profit"`

	sync.Mutex
}

// NewProfit generates the profit object from the current position
func (p *Position) NewProfit(trade Trade, profit, netProfit fixedpoint.Value) Profit {
	return Profit{
		Symbol:        p.Symbol,
		QuoteCurrency: p.QuoteCurrency,
		BaseCurrency:  p.BaseCurrency,
		AverageCost:   p.AverageCost,
		// profit related fields
		Profit:          profit,
		NetProfit:       netProfit,
		ProfitMargin:    profit.Div(trade.QuoteQuantity),
		NetProfitMargin: netProfit.Div(trade.QuoteQuantity),
		// trade related fields
		TradeID:       trade.ID,
		Side:          trade.Side,
		IsBuyer:       trade.IsBuyer,
		IsMaker:       trade.IsMaker,
		Price:         trade.Price,
		Quantity:      trade.Quantity,
		QuoteQuantity: trade.QuoteQuantity,
		// FeeInUSD:           0,
		Fee:         trade.Fee,
		FeeCurrency: trade.FeeCurrency,

		Exchange:   trade.Exchange,
		IsMargin:   trade.IsMargin,
		IsFutures:  trade.IsFutures,
		IsIsolated: trade.IsIsolated,
		TradedAt:   trade.Time.Time(),
	}
}

func (p *Position) NewClosePositionOrder(percentage fixedpoint.Value) *SubmitOrder {
	base := p.GetBase()
	quantity := base.Mul(percentage)
	if quantity.Compare(p.Market.MinQuantity) < 0 {
		return nil
	}

	side := SideTypeSell
	sign := base.Sign()
	if sign == 0 {
		return nil
	} else if sign < 0 {
		side = SideTypeBuy
	}

	return &SubmitOrder{
		Symbol:   p.Symbol,
		Market:   p.Market,
		Type:     OrderTypeMarket,
		Side:     side,
		Quantity: quantity,
	}
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
	p.TotalFee[trade.FeeCurrency] = p.TotalFee[trade.FeeCurrency].Add(trade.Fee)
}

func (p *Position) Reset() {
	p.Base = fixedpoint.Zero
	p.Quote = fixedpoint.Zero
	p.AverageCost = fixedpoint.Zero
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
	if p.Base.Sign() > 0 {
		return PositionLong
	} else if p.Base.Sign() < 0 {
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

	sign := p.Base.Sign()
	if sign == 0 {
		color = "#cccccc"
	} else if sign > 0 {
		color = "#228B22"
	} else if sign < 0 {
		color = "#DC143C"
	}

	title := util.Render(string(posType)+` Position {{ .Symbol }} `, p)

	fields := []slack.AttachmentField{
		{Title: "Average Cost", Value: averageCost.String() + " " + p.QuoteCurrency, Short: true},
		{Title: p.BaseCurrency, Value: base.String(), Short: true},
		{Title: p.QuoteCurrency, Value: quote.String()},
	}

	if p.TotalFee != nil {
		for feeCurrency, fee := range p.TotalFee {
			if fee.Sign() > 0 {
				fields = append(fields, slack.AttachmentField{
					Title: fmt.Sprintf("Fee (%s)", feeCurrency),
					Value: fee.String(),
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
	msg = fmt.Sprintf("%s Position %s: average cost = %v, base = %v, quote = %v",
		posType,
		p.Symbol,
		p.AverageCost,
		p.Base,
		p.Quote,
	)

	if p.TotalFee != nil {
		for feeCurrency, fee := range p.TotalFee {
			msg += fmt.Sprintf("\nfee (%s) = %v", feeCurrency, fee)
		}
	}

	return msg
}

func (p *Position) String() string {
	return fmt.Sprintf("POSITION %s: average cost = %v, base = %v, quote = %v",
		p.Symbol,
		p.AverageCost,
		p.Base,
		p.Quote,
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
			totalProfitAmount = totalProfitAmount.Add(profit)
			totalNetProfit = totalNetProfit.Add(netProfit)
		}
	}

	return totalProfitAmount, totalNetProfit, !totalProfitAmount.IsZero()
}

func (p *Position) AddTrade(td Trade) (profit fixedpoint.Value, netProfit fixedpoint.Value, madeProfit bool) {
	price := td.Price
	quantity := td.Quantity
	quoteQuantity := td.QuoteQuantity
	fee := td.Fee

	// calculated fee in quote (some exchange accounts may enable platform currency fee discount, like BNB)
	// convert platform fee token into USD values
	var feeInQuote fixedpoint.Value = fixedpoint.Zero

	switch td.FeeCurrency {

	case p.BaseCurrency:
		quantity = quantity.Sub(fee)

	case p.QuoteCurrency:
		quoteQuantity = quoteQuantity.Sub(fee)

	default:
		if p.ExchangeFeeRates != nil {
			if exchangeFee, ok := p.ExchangeFeeRates[td.Exchange]; ok {
				if td.IsMaker {
					feeInQuote = feeInQuote.Add(exchangeFee.MakerFeeRate.Mul(quoteQuantity))
				} else {
					feeInQuote = feeInQuote.Add(exchangeFee.TakerFeeRate.Mul(quoteQuantity))
				}
			}
		} else if p.FeeRate != nil {
			if td.IsMaker {
				feeInQuote = feeInQuote.Add(p.FeeRate.MakerFeeRate.Mul(quoteQuantity))
			} else {
				feeInQuote = feeInQuote.Add(p.FeeRate.TakerFeeRate.Mul(quoteQuantity))
			}
		}
	}

	p.Lock()
	defer p.Unlock()

	// update changedAt field before we unlock in the defer func
	defer func() {
		p.ChangedAt = td.Time.Time()
	}()

	p.addTradeFee(td)

	// Base > 0 means we're in long position
	// Base < 0  means we're in short position
	switch td.Side {

	case SideTypeBuy:
		if p.Base.Sign() < 0 {
			// convert short position to long position
			if p.Base.Add(quantity).Sign() > 0 {
				profit = p.AverageCost.Sub(price).Mul(p.Base.Neg())
				netProfit = p.ApproximateAverageCost.Sub(price).Mul(p.Base.Neg()).Sub(feeInQuote)
				p.Base = p.Base.Add(quantity)
				p.Quote = p.Quote.Sub(quoteQuantity)
				p.AverageCost = price
				p.ApproximateAverageCost = price
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			} else {
				// covering short position
				p.Base = p.Base.Add(quantity)
				p.Quote = p.Quote.Sub(quoteQuantity)
				profit = p.AverageCost.Sub(price).Mul(quantity)
				netProfit = p.ApproximateAverageCost.Sub(price).Mul(quantity).Sub(feeInQuote)
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			}
		}

		divisor := p.Base.Add(quantity)
		p.ApproximateAverageCost = p.ApproximateAverageCost.Mul(p.Base).
			Add(quoteQuantity).
			Add(feeInQuote).
			Div(divisor)
		p.AverageCost = p.AverageCost.Mul(p.Base).Add(quoteQuantity).Div(divisor)
		p.Base = p.Base.Add(quantity)
		p.Quote = p.Quote.Sub(quoteQuantity)

		return fixedpoint.Zero, fixedpoint.Zero, false

	case SideTypeSell:
		if p.Base.Sign() > 0 {
			// convert long position to short position
			if p.Base.Compare(quantity) < 0 {
				profit = price.Sub(p.AverageCost).Mul(p.Base)
				netProfit = price.Sub(p.ApproximateAverageCost).Mul(p.Base).Sub(feeInQuote)
				p.Base = p.Base.Sub(quantity)
				p.Quote = p.Quote.Add(quoteQuantity)
				p.AverageCost = price
				p.ApproximateAverageCost = price
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			} else {
				p.Base = p.Base.Sub(quantity)
				p.Quote = p.Quote.Add(quoteQuantity)
				profit = price.Sub(p.AverageCost).Mul(quantity)
				netProfit = price.Sub(p.ApproximateAverageCost).Mul(quantity).Sub(feeInQuote)
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			}
		}

		// handling short position, since Base here is negative we need to reverse the sign
		divisor := quantity.Sub(p.Base)
		p.ApproximateAverageCost = p.ApproximateAverageCost.Mul(p.Base.Neg()).
			Add(quoteQuantity).
			Sub(feeInQuote).
			Div(divisor)

		p.AverageCost = p.AverageCost.Mul(p.Base.Neg()).
			Add(quoteQuantity).
			Div(divisor)
		p.Base = p.Base.Sub(quantity)
		p.Quote = p.Quote.Add(quoteQuantity)

		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	return fixedpoint.Zero, fixedpoint.Zero, false
}
