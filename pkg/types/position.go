package types

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util/templateutil"
)

type PositionType string

const (
	PositionShort  = PositionType("Short")
	PositionLong   = PositionType("Long")
	PositionClosed = PositionType("Closed")
)

// ExchangeFee stores the exchange fee rate
type ExchangeFee struct {
	MakerFeeRate fixedpoint.Value
	TakerFeeRate fixedpoint.Value
}

// PositionRisk stores the position risk data
type PositionRisk struct {
	Leverage         fixedpoint.Value `json:"leverage,omitempty"`
	LiquidationPrice fixedpoint.Value `json:"liquidationPrice,omitempty"`
}

// Position stores the position data
type Position struct {
	Symbol        string `json:"symbol" db:"symbol"`
	BaseCurrency  string `json:"baseCurrency" db:"base"`
	QuoteCurrency string `json:"quoteCurrency" db:"quote"`

	Market Market `json:"market,omitempty"`

	Base        fixedpoint.Value `json:"base" db:"base"`
	Quote       fixedpoint.Value `json:"quote" db:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost" db:"average_cost"`

	FeeRate          *ExchangeFee                 `json:"feeRate,omitempty"`
	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	// TotalFee stores the fee currency -> total fee quantity
	TotalFee map[string]fixedpoint.Value `json:"totalFee" db:"-"`

	// FeeAverageCosts stores the fee currency -> average cost of the fee
	// e.g. BNB -> 341.0
	FeeAverageCosts map[string]fixedpoint.Value `json:"feeAverageCosts" db:"-"`

	OpenedAt  time.Time `json:"openedAt,omitempty" db:"-"`
	ChangedAt time.Time `json:"changedAt,omitempty" db:"changed_at"`

	Strategy           string `json:"strategy,omitempty" db:"strategy"`
	StrategyInstanceID string `json:"strategyInstanceID,omitempty" db:"strategy_instance_id"`

	AccumulatedProfit fixedpoint.Value `json:"accumulatedProfit,omitempty" db:"accumulated_profit"`

	// closing is a flag for marking this position is closing
	closing bool

	sync.Mutex

	// Modify position callbacks
	modifyCallbacks []func(baseQty fixedpoint.Value, quoteQty fixedpoint.Value, price fixedpoint.Value)

	// ttl is the ttl to keep in persistence
	ttl time.Duration
}

func (s *Position) SetTTL(ttl time.Duration) {
	if ttl.Nanoseconds() <= 0 {
		return
	}
	s.ttl = ttl
}

func (s *Position) Expiration() time.Duration {
	return s.ttl
}

func (p *Position) CsvHeader() []string {
	return []string{
		"symbol",
		"time",
		"average_cost",
		"base",
		"quote",
		"accumulated_profit",
	}
}

func (p *Position) CsvRecords() [][]string {
	if p.AverageCost.IsZero() && p.Base.IsZero() {
		return nil
	}

	return [][]string{
		{
			p.Symbol,
			p.ChangedAt.UTC().Format(time.RFC1123),
			p.AverageCost.String(),
			p.Base.String(),
			p.Quote.String(),
			p.AccumulatedProfit.String(),
		},
	}
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
		Trade:         &trade,
		TradeID:       trade.ID,
		OrderID:       trade.OrderID,
		Side:          trade.Side,
		IsBuyer:       trade.IsBuyer,
		IsMaker:       trade.IsMaker,
		Price:         trade.Price,
		Quantity:      trade.Quantity,
		QuoteQuantity: trade.QuoteQuantity,
		// FeeInUSD:           0,
		Fee:         trade.Fee,
		FeeCurrency: trade.FeeCurrency,

		Exchange:           trade.Exchange,
		IsMargin:           trade.IsMargin,
		IsFutures:          trade.IsFutures,
		IsIsolated:         trade.IsIsolated,
		TradedAt:           trade.Time.Time(),
		Strategy:           p.Strategy,
		StrategyInstanceID: p.StrategyInstanceID,

		PositionOpenedAt: p.OpenedAt,
	}
}

// ROI -- Return on investment (ROI) is a performance measure used to evaluate the efficiency or profitability of an investment
// or compare the efficiency of a number of different investments.
// ROI tries to directly measure the amount of return on a particular investment, relative to the investment's cost.
func (p *Position) ROI(price fixedpoint.Value) fixedpoint.Value {
	unrealizedProfit := p.UnrealizedProfit(price)
	cost := p.AverageCost.Mul(p.Base.Abs())
	return unrealizedProfit.Div(cost)
}

func (p *Position) NewMarketCloseOrder(percentage fixedpoint.Value) *SubmitOrder {
	base := p.GetBase()

	quantity := base.Abs()
	if percentage.Compare(fixedpoint.One) < 0 {
		quantity = quantity.Mul(percentage)
	}

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
		Symbol:           p.Symbol,
		Market:           p.Market,
		Type:             OrderTypeMarket,
		Side:             side,
		Quantity:         quantity,
		MarginSideEffect: SideEffectTypeAutoRepay,
	}
}

// IsDust checks if the position is dust, the first argument is the price to calculate the dust quantity
func (p *Position) IsDust(a ...fixedpoint.Value) bool {
	price := p.AverageCost
	if len(a) > 0 {
		price = a[0]
	}

	base := p.Base.Abs()
	return p.Market.IsDustQuantity(base, price)
}

// GetBase locks the mutex and return the base quantity
// The base quantity can be negative
func (p *Position) GetBase() (base fixedpoint.Value) {
	p.Lock()
	base = p.Base
	p.Unlock()
	return base
}

// GetQuantity calls GetBase() and then convert the number into a positive number
// that could be treated as a quantity.
func (p *Position) GetQuantity() fixedpoint.Value {
	base := p.GetBase()
	return base.Abs()
}

func (p *Position) UnrealizedProfit(price fixedpoint.Value) fixedpoint.Value {
	quantity := p.GetBase().Abs()

	if p.IsLong() {
		return price.Sub(p.AverageCost).Mul(quantity)
	} else if p.IsShort() {
		return p.AverageCost.Sub(price).Mul(quantity)
	}

	return fixedpoint.Zero
}

func (p *Position) OnModify(cb func(baseQty fixedpoint.Value, quoteQty fixedpoint.Value, price fixedpoint.Value)) {
	p.modifyCallbacks = append(p.modifyCallbacks, cb)
}

func (p *Position) EmitModify(baseQty fixedpoint.Value, quoteQty fixedpoint.Value, price fixedpoint.Value) {
	for _, cb := range p.modifyCallbacks {
		cb(baseQty, quoteQty, price)
	}
}

// ModifyBase modifies position base quantity with `qty`
func (p *Position) ModifyBase(qty fixedpoint.Value) error {
	p.Base = qty

	p.EmitModify(p.Base, p.Quote, p.AverageCost)

	return nil
}

// ModifyQuote modifies position quote quantity with `qty`
func (p *Position) ModifyQuote(qty fixedpoint.Value) error {
	p.Quote = qty

	p.EmitModify(p.Base, p.Quote, p.AverageCost)

	return nil
}

// ModifyAverageCost modifies position average cost with `price`
func (p *Position) ModifyAverageCost(price fixedpoint.Value) error {
	p.AverageCost = price

	p.EmitModify(p.Base, p.Quote, p.AverageCost)

	return nil
}

type FuturesPosition struct {
	Symbol        string `json:"symbol"`
	BaseCurrency  string `json:"baseCurrency"`
	QuoteCurrency string `json:"quoteCurrency"`

	Market Market `json:"market"`

	Base        fixedpoint.Value `json:"base"`
	Quote       fixedpoint.Value `json:"quote"`
	AverageCost fixedpoint.Value `json:"averageCost"`

	FeeRate          *ExchangeFee                 `json:"feeRate,omitempty"`
	ExchangeFeeRates map[ExchangeName]ExchangeFee `json:"exchangeFeeRates"`

	// Futures data fields
	// -------------------
	// Isolated margin mode
	Isolated bool `json:"isolated"`

	// UpdateTime is the time when the position is updated
	UpdateTime int64 `json:"updateTime"`

	// PositionRisk stores the position risk data
	PositionRisk *PositionRisk
}

func NewPositionFromMarket(market Market) *Position {
	if len(market.BaseCurrency) == 0 || len(market.QuoteCurrency) == 0 {
		panic("logical exception: missing market information, base currency or quote currency is empty")
	}

	return &Position{
		Symbol:        market.Symbol,
		BaseCurrency:  market.BaseCurrency,
		QuoteCurrency: market.QuoteCurrency,
		Market:        market,

		FeeAverageCosts:  make(map[string]fixedpoint.Value),
		TotalFee:         make(map[string]fixedpoint.Value),
		ExchangeFeeRates: make(map[ExchangeName]ExchangeFee),
	}
}

func NewPosition(symbol, base, quote string) *Position {
	return &Position{
		Symbol:        symbol,
		BaseCurrency:  base,
		QuoteCurrency: quote,

		TotalFee:         make(map[string]fixedpoint.Value),
		FeeAverageCosts:  make(map[string]fixedpoint.Value),
		ExchangeFeeRates: make(map[ExchangeName]ExchangeFee),
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
	p.TotalFee = make(map[string]fixedpoint.Value)
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

func (p *Position) SetFeeAverageCost(currency string, cost fixedpoint.Value) {
	p.FeeAverageCosts[currency] = cost
}

func (p *Position) IsShort() bool {
	return p.Base.Sign() < 0
}

func (p *Position) IsLong() bool {
	return p.Base.Sign() > 0
}

func (p *Position) IsClosed() bool {
	return p.Base.Sign() == 0
}

func (p *Position) IsOpened(currentPrice fixedpoint.Value) bool {
	return !p.IsClosed() && !p.IsDust(currentPrice)
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

	title := templateutil.Render(string(posType)+` Position {{ .Symbol }} `, p)

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
		Footer: templateutil.Render("update time {{ . }}", time.Now().Format(time.RFC822)),
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

// BindStream binds the trade update callback and update the position
func (p *Position) BindStream(stream Stream) {
	stream.OnTradeUpdate(func(trade Trade) {
		if p.Symbol == trade.Symbol {
			p.AddTrade(trade)
		}
	})
}

func (p *Position) SetClosing(c bool) bool {
	p.Lock()
	defer p.Unlock()

	if p.closing && c {
		return false
	}

	p.closing = c
	return true
}

func (p *Position) IsClosing() (c bool) {
	p.Lock()
	c = p.closing
	p.Unlock()
	return c
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

func (p *Position) calculateFeeInQuote(td Trade) fixedpoint.Value {
	var quoteQuantity = td.QuoteQuantity

	if cost, ok := p.FeeAverageCosts[td.FeeCurrency]; ok {
		return td.Fee.Mul(cost)
	}

	if p.ExchangeFeeRates != nil {
		if exchangeFee, ok := p.ExchangeFeeRates[td.Exchange]; ok {
			if td.IsMaker {
				return exchangeFee.MakerFeeRate.Mul(quoteQuantity)
			} else {
				return exchangeFee.TakerFeeRate.Mul(quoteQuantity)
			}
		}
	}

	if p.FeeRate != nil {
		if td.IsMaker {
			return p.FeeRate.MakerFeeRate.Mul(quoteQuantity)
		} else {
			return p.FeeRate.TakerFeeRate.Mul(quoteQuantity)
		}
	}

	return fixedpoint.Zero
}

func (p *Position) AddTrade(td Trade) (profit fixedpoint.Value, netProfit fixedpoint.Value, madeProfit bool) {
	price := td.Price
	quantity := td.Quantity
	quoteQuantity := td.QuoteQuantity
	fee := td.Fee

	// calculated fee in quote (some exchange accounts may enable platform currency fee discount, like BNB)
	// convert platform fee token into USD values
	var feeInQuote = fixedpoint.Zero

	switch td.FeeCurrency {

	case p.BaseCurrency:
		// USD-M futures use the quote currency as the fee currency.
		if !td.IsFutures {
			quantity = quantity.Sub(fee)
		}

	case p.QuoteCurrency:
		if !td.IsFutures {
			quoteQuantity = quoteQuantity.Sub(fee)
		}

	default:
		if !td.Fee.IsZero() {
			feeInQuote = p.calculateFeeInQuote(td)
		}
	}

	p.Lock()
	defer p.Unlock()

	defer p.updateMetrics()

	// update changedAt field before we unlock in the defer func
	defer func() {
		p.ChangedAt = td.Time.Time()
	}()

	p.addTradeFee(td)

	// Base > 0 means we're in long position
	// Base < 0 means we're in short position
	switch td.Side {

	case SideTypeBuy:
		// was short position, now trade buy should cover the position
		if p.Base.Sign() < 0 {
			// convert short position to long position
			if p.Base.Add(quantity).Sign() > 0 {
				profit = p.AverageCost.Sub(price).Mul(p.Base.Neg())
				netProfit = p.AverageCost.Sub(price).Mul(p.Base.Neg()).Sub(feeInQuote)
				p.Base = p.Base.Add(quantity)
				p.Quote = p.Quote.Sub(quoteQuantity)
				p.AverageCost = price
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				p.OpenedAt = td.Time.Time()
				return profit, netProfit, true
			} else {
				// after adding quantity it's still short position
				p.Base = p.Base.Add(quantity)
				p.Quote = p.Quote.Sub(quoteQuantity)
				profit = p.AverageCost.Sub(price).Mul(quantity)
				netProfit = p.AverageCost.Sub(price).Mul(quantity).Sub(feeInQuote)
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			}
		}

		// before adding the quantity, it's already a dust position
		// then we should set the openedAt time
		if p.IsDust(td.Price) {
			p.OpenedAt = td.Time.Time()
		}

		// here the case is: base == 0 or base > 0
		divisor := p.Base.Add(quantity)

		p.AverageCost = p.AverageCost.Mul(p.Base).
			Add(quoteQuantity).
			Add(feeInQuote).
			Div(divisor)

		p.Base = p.Base.Add(quantity)
		p.Quote = p.Quote.Sub(quoteQuantity)
		return fixedpoint.Zero, fixedpoint.Zero, false

	case SideTypeSell:
		// was long position, the sell trade should reduce the base amount
		if p.Base.Sign() > 0 {
			// convert long position to short position
			if p.Base.Compare(quantity) < 0 {
				profit = price.Sub(p.AverageCost).Mul(p.Base)
				netProfit = price.Sub(p.AverageCost).Mul(p.Base).Sub(feeInQuote)
				p.Base = p.Base.Sub(quantity)
				p.Quote = p.Quote.Add(quoteQuantity)
				p.AverageCost = price
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				p.OpenedAt = td.Time.Time()
				return profit, netProfit, true
			} else {
				p.Base = p.Base.Sub(quantity)
				p.Quote = p.Quote.Add(quoteQuantity)
				profit = price.Sub(p.AverageCost).Mul(quantity)
				netProfit = price.Sub(p.AverageCost).Mul(quantity).Sub(feeInQuote)
				p.AccumulatedProfit = p.AccumulatedProfit.Add(profit)
				return profit, netProfit, true
			}
		}

		// before subtracting the quantity, it's already a dust position
		// then we should set the openedAt time
		if p.IsDust(td.Price) {
			p.OpenedAt = td.Time.Time()
		}

		// handling short position, since Base here is negative we need to reverse the sign
		divisor := quantity.Sub(p.Base)

		p.AverageCost = p.AverageCost.Mul(p.Base.Neg()).
			Add(quoteQuantity).
			Sub(feeInQuote).
			Div(divisor)
		p.Base = p.Base.Sub(quantity)
		p.Quote = p.Quote.Add(quoteQuantity)

		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	return fixedpoint.Zero, fixedpoint.Zero, false
}

func (p *Position) UpdateMetrics() {
	p.Lock()
	p.updateMetrics()
	p.Unlock()
}

func (p *Position) updateMetrics() {
	// update the position metrics only if the position defines the strategy ID
	if p.StrategyInstanceID == "" || p.Strategy == "" {
		return
	}

	labels := prometheus.Labels{
		"strategy_id":   p.StrategyInstanceID,
		"strategy_type": p.Strategy,
		"symbol":        p.Symbol,
	}
	positionAverageCostMetrics.With(labels).Set(p.AverageCost.Float64())
	positionBaseQuantityMetrics.With(labels).Set(p.Base.Float64())
	positionQuoteQuantityMetrics.With(labels).Set(p.Quote.Float64())
}
