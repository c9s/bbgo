package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("risk", "AccountValueCalculator")

var one = fixedpoint.One

var defaultLeverage = fixedpoint.NewFromInt(3)

var maxLeverage = fixedpoint.NewFromInt(10)

type AccountValueCalculator struct {
	session       *bbgo.ExchangeSession
	quoteCurrency string
	prices        map[string]fixedpoint.Value
	tickers       map[string]types.Ticker
	updateTime    time.Time
}

func NewAccountValueCalculator(session *bbgo.ExchangeSession, quoteCurrency string) *AccountValueCalculator {
	return &AccountValueCalculator{
		session:       session,
		quoteCurrency: quoteCurrency,
		prices:        make(map[string]fixedpoint.Value),
		tickers:       make(map[string]types.Ticker),
	}
}

func (c *AccountValueCalculator) UpdatePrices(ctx context.Context) error {
	balances := c.session.Account.Balances()
	currencies := balances.Currencies()
	var symbols []string
	for _, currency := range currencies {
		if currency == c.quoteCurrency {
			continue
		}

		symbol := currency + c.quoteCurrency
		symbols = append(symbols, symbol)
	}

	tickers, err := c.session.Exchange.QueryTickers(ctx, symbols...)
	if err != nil {
		return err
	}

	c.tickers = tickers
	for symbol, ticker := range tickers {
		c.prices[symbol] = ticker.Last
		if ticker.Time.After(c.updateTime) {
			c.updateTime = ticker.Time
		}
	}
	return nil
}

func (c *AccountValueCalculator) DebtValue(ctx context.Context) (fixedpoint.Value, error) {
	debtValue := fixedpoint.Zero

	if len(c.prices) == 0 {
		if err := c.UpdatePrices(ctx); err != nil {
			return debtValue, err
		}
	}

	balances := c.session.Account.Balances()
	for _, b := range balances {
		symbol := b.Currency + c.quoteCurrency
		price, ok := c.prices[symbol]
		if !ok {
			continue
		}

		debtValue = debtValue.Add(b.Debt().Mul(price))
	}

	return debtValue, nil
}

func (c *AccountValueCalculator) MarketValue(ctx context.Context) (fixedpoint.Value, error) {
	marketValue := fixedpoint.Zero

	if len(c.prices) == 0 {
		if err := c.UpdatePrices(ctx); err != nil {
			return marketValue, err
		}
	}

	balances := c.session.Account.Balances()
	for _, b := range balances {
		if b.Currency == c.quoteCurrency {
			marketValue = marketValue.Add(b.Total())
			continue
		}

		symbol := b.Currency + c.quoteCurrency
		price, ok := c.prices[symbol]
		if !ok {
			continue
		}

		marketValue = marketValue.Add(b.Total().Mul(price))
	}

	return marketValue, nil
}

func (c *AccountValueCalculator) NetValue(ctx context.Context) (fixedpoint.Value, error) {
	accountValue := fixedpoint.Zero

	if len(c.prices) == 0 {
		if err := c.UpdatePrices(ctx); err != nil {
			return accountValue, err
		}
	}

	balances := c.session.Account.Balances()
	for _, b := range balances {
		if b.Currency == c.quoteCurrency {
			accountValue = accountValue.Add(b.Net())
			continue
		}

		symbol := b.Currency + c.quoteCurrency
		price, ok := c.prices[symbol]
		if !ok {
			continue
		}

		accountValue = accountValue.Add(b.Net().Mul(price))
	}

	return accountValue, nil
}

func (c *AccountValueCalculator) AvailableQuote(ctx context.Context) (fixedpoint.Value, error) {
	accountValue := fixedpoint.Zero

	if len(c.prices) == 0 {
		if err := c.UpdatePrices(ctx); err != nil {
			return accountValue, err
		}
	}

	balances := c.session.Account.Balances()
	for _, b := range balances {
		if b.Currency == c.quoteCurrency {
			accountValue = accountValue.Add(b.Net())
			continue
		}

		symbol := b.Currency + c.quoteCurrency
		price, ok := c.prices[symbol]
		if !ok {
			continue
		}

		accountValue = accountValue.Add(b.Net().Mul(price))
	}

	return accountValue, nil
}

// MarginLevel calculates the margin level from the asset market value and the debt value
// See https://www.binance.com/en/support/faq/360030493931
func (c *AccountValueCalculator) MarginLevel(ctx context.Context) (fixedpoint.Value, error) {
	marginLevel := fixedpoint.Zero
	marketValue, err := c.MarketValue(ctx)
	if err != nil {
		return marginLevel, err
	}

	debtValue, err := c.DebtValue(ctx)
	if err != nil {
		return marginLevel, err
	}

	marginLevel = marketValue.Div(debtValue)
	return marginLevel, nil
}

func CalculateBaseQuantity(session *bbgo.ExchangeSession, market types.Market, price, quantity, leverage fixedpoint.Value) (fixedpoint.Value, error) {
	// default leverage guard
	if leverage.IsZero() {
		leverage = defaultLeverage
	}

	baseBalance, _ := session.Account.Balance(market.BaseCurrency)
	quoteBalance, _ := session.Account.Balance(market.QuoteCurrency)

	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if !usingLeverage {
		// For spot, we simply sell the base quoteCurrency
		balance, hasBalance := session.Account.Balance(market.BaseCurrency)
		if hasBalance {
			if quantity.IsZero() {
				logrus.Warnf("sell quantity is not set, using all available base balance: %v", balance)
				if !balance.Available.IsZero() {
					return balance.Available, nil
				}
			} else {
				return fixedpoint.Min(quantity, balance.Available), nil
			}
		}

		return quantity, fmt.Errorf("quantity is zero, can not submit sell order, please check your quantity settings")
	}

	if !quantity.IsZero() {
		return quantity, nil
	}

	// using leverage -- starts from here
	logrus.Infof("calculating available leveraged base quantity: base balance = %+v, quote balance = %+v", baseBalance, quoteBalance)

	// calculate the quantity automatically
	if session.Margin || session.IsolatedMargin {
		baseBalanceValue := baseBalance.Net().Mul(price)
		accountValue := baseBalanceValue.Add(quoteBalance.Net())

		// avoid using all account value since there will be some trade loss for interests and the fee
		accountValue = accountValue.Mul(one.Sub(fixedpoint.NewFromFloat(0.01)))

		logrus.Infof("calculated account value %f %s", accountValue.Float64(), market.QuoteCurrency)

		if session.IsolatedMargin {
			originLeverage := leverage
			leverage = fixedpoint.Min(leverage, maxLeverage)
			logrus.Infof("using isolated margin, maxLeverage=10 originalLeverage=%f currentLeverage=%f",
				originLeverage.Float64(),
				leverage.Float64())
		}

		// spot margin use the equity value, so we use the total quote balance here
		maxPosition := CalculateMaxPosition(price, accountValue, leverage)
		debt := baseBalance.Debt()
		maxQuantity := maxPosition.Sub(debt)

		logrus.Infof("margin leverage: calculated maxQuantity=%f maxPosition=%f debt=%f price=%f accountValue=%f %s leverage=%f",
			maxQuantity.Float64(),
			maxPosition.Float64(),
			debt.Float64(),
			price.Float64(),
			accountValue.Float64(),
			market.QuoteCurrency,
			leverage.Float64())

		return maxQuantity, nil
	}

	if session.Futures || session.IsolatedFutures {
		// TODO: get mark price here
		maxPositionQuantity := CalculateMaxPosition(price, quoteBalance.Available, leverage)
		requiredPositionCost := CalculatePositionCost(price, price, maxPositionQuantity, leverage, types.SideTypeSell)
		if quoteBalance.Available.Compare(requiredPositionCost) < 0 {
			return maxPositionQuantity, fmt.Errorf("available margin %f %s is not enough, can not submit order", quoteBalance.Available.Float64(), market.QuoteCurrency)
		}

		return maxPositionQuantity, nil
	}

	return quantity, fmt.Errorf("quantity is zero, can not submit sell order, please check your settings")
}

func CalculateQuoteQuantity(ctx context.Context, session *bbgo.ExchangeSession, quoteCurrency string, leverage fixedpoint.Value) (fixedpoint.Value, error) {
	// default leverage guard
	if leverage.IsZero() {
		leverage = defaultLeverage
	}

	quoteBalance, _ := session.Account.Balance(quoteCurrency)
	accountValue := NewAccountValueCalculator(session, quoteCurrency)

	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if !usingLeverage {
		// For spot, we simply return the quote balance
		return quoteBalance.Available.Mul(fixedpoint.Min(leverage, fixedpoint.One)), nil
	}

	// using leverage -- starts from here
	availableQuote, err := accountValue.AvailableQuote(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update available quote")
		return fixedpoint.Zero, err
	}
	logrus.Infof("calculating available leveraged quote quantity: account available quote = %+v", availableQuote)

	return availableQuote.Mul(leverage), nil
}
