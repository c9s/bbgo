package bbgo

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/risk"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultLeverage = fixedpoint.NewFromInt(3)

var maxIsolatedMarginLeverage = fixedpoint.NewFromInt(10)

var maxCrossMarginLeverage = fixedpoint.NewFromInt(3)

type AccountValueCalculator struct {
	session       *ExchangeSession
	quoteCurrency string
	prices        map[string]fixedpoint.Value
	tickers       map[string]types.Ticker
	updateTime    time.Time
}

func NewAccountValueCalculator(session *ExchangeSession, quoteCurrency string) *AccountValueCalculator {
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

func aggregateUsdValue(balances types.BalanceMap) fixedpoint.Value {
	totalUsdValue := fixedpoint.Zero
	// get all usd value if any
	for currency, balance := range balances {
		if types.IsUSDFiatCurrency(currency) {
			totalUsdValue = totalUsdValue.Add(balance.Net())
		}
	}

	return totalUsdValue
}

func CalculateBaseQuantity(session *ExchangeSession, market types.Market, price, quantity, leverage fixedpoint.Value) (fixedpoint.Value, error) {
	// default leverage guard
	if leverage.IsZero() {
		leverage = defaultLeverage
	}

	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if !usingLeverage {
		// For spot, we simply sell the base quoteCurrency
		balance, hasBalance := session.Account.Balance(market.BaseCurrency)
		if hasBalance {
			if quantity.IsZero() {
				log.Warnf("sell quantity is not set, using all available base balance: %v", balance)
				if !balance.Available.IsZero() {
					return balance.Available, nil
				}
			} else {
				return fixedpoint.Min(quantity, balance.Available), nil
			}
		}

		return quantity, fmt.Errorf("quantity is zero, can not submit sell order, please check your quantity settings")
	}

	baseBalance, _ := session.Account.Balance(market.BaseCurrency)
	quoteBalance, _ := session.Account.Balance(market.QuoteCurrency)
	balances := session.Account.Balances()

	// for isolated margin we can calculate from these two pair
	totalUsdValue := fixedpoint.Zero
	if types.IsUSDFiatCurrency(market.QuoteCurrency) {
		totalUsdValue = aggregateUsdValue(balances)
	} else {
		// TODO: translate quote currency like BTC of ETH/BTC to usd value
		totalUsdValue = quoteBalance.Net()
	}

	if !quantity.IsZero() {
		return quantity, nil
	}

	if price.IsZero() {
		return quantity, fmt.Errorf("%s price can not be zero", market.Symbol)
	}

	// using leverage -- starts from here
	log.Infof("calculating available leveraged base quantity: base balance = %+v, quote balance = %+v", baseBalance, quoteBalance)

	// calculate the quantity automatically
	if session.Margin || session.IsolatedMargin {
		baseBalanceValue := baseBalance.Net().Mul(price)
		accountUsdValue := baseBalanceValue.Add(totalUsdValue)

		// avoid using all account value since there will be some trade loss for interests and the fee
		accountUsdValue = accountUsdValue.Mul(one.Sub(fixedpoint.NewFromFloat(0.01)))

		log.Infof("calculated account usd value %f %s", accountUsdValue.Float64(), market.QuoteCurrency)

		originLeverage := leverage
		if session.IsolatedMargin {
			leverage = fixedpoint.Min(leverage, maxIsolatedMarginLeverage)
			log.Infof("using isolated margin, maxLeverage=%f originalLeverage=%f currentLeverage=%f",
				maxIsolatedMarginLeverage.Float64(),
				originLeverage.Float64(),
				leverage.Float64())
		} else {
			leverage = fixedpoint.Min(leverage, maxCrossMarginLeverage)
			log.Infof("using cross margin, maxLeverage=%f originalLeverage=%f currentLeverage=%f",
				maxCrossMarginLeverage.Float64(),
				originLeverage.Float64(),
				leverage.Float64())
		}

		// spot margin use the equity value, so we use the total quote balance here
		maxPosition := risk.CalculateMaxPosition(price, accountUsdValue, leverage)
		debt := baseBalance.Debt()
		maxQuantity := maxPosition.Sub(debt)

		log.Infof("margin leverage: calculated maxQuantity=%f maxPosition=%f debt=%f price=%f accountValue=%f %s leverage=%f",
			maxQuantity.Float64(),
			maxPosition.Float64(),
			debt.Float64(),
			price.Float64(),
			accountUsdValue.Float64(),
			market.QuoteCurrency,
			leverage.Float64())

		return maxQuantity, nil
	}

	if session.Futures || session.IsolatedFutures {
		// TODO: get mark price here
		maxPositionQuantity := risk.CalculateMaxPosition(price, totalUsdValue, leverage)
		requiredPositionCost := risk.CalculatePositionCost(price, price, maxPositionQuantity, leverage, types.SideTypeSell)
		if quoteBalance.Available.Compare(requiredPositionCost) < 0 {
			return maxPositionQuantity, fmt.Errorf("margin total usd value %f is not enough, can not submit order", totalUsdValue.Float64())
		}

		return maxPositionQuantity, nil
	}

	return quantity, fmt.Errorf("quantity is zero, can not submit sell order, please check your settings")
}

func CalculateQuoteQuantity(ctx context.Context, session *ExchangeSession, quoteCurrency string, leverage fixedpoint.Value) (fixedpoint.Value, error) {
	// default leverage guard
	if leverage.IsZero() {
		leverage = defaultLeverage
	}

	quoteBalance, _ := session.Account.Balance(quoteCurrency)

	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if !usingLeverage {
		// For spot, we simply return the quote balance
		return quoteBalance.Available.Mul(fixedpoint.Min(leverage, fixedpoint.One)), nil
	}

	originLeverage := leverage
	if session.IsolatedMargin {
		leverage = fixedpoint.Min(leverage, maxIsolatedMarginLeverage)
		log.Infof("using isolated margin, maxLeverage=%f originalLeverage=%f currentLeverage=%f",
			maxIsolatedMarginLeverage.Float64(),
			originLeverage.Float64(),
			leverage.Float64())
	} else {
		leverage = fixedpoint.Min(leverage, maxCrossMarginLeverage)
		log.Infof("using cross margin, maxLeverage=%f originalLeverage=%f currentLeverage=%f",
			maxCrossMarginLeverage.Float64(),
			originLeverage.Float64(),
			leverage.Float64())
	}

	// using leverage -- starts from here
	accountValue := NewAccountValueCalculator(session, quoteCurrency)
	availableQuote, err := accountValue.AvailableQuote(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update available quote")
		return fixedpoint.Zero, err
	}

	log.Infof("calculating available leveraged quote quantity: account available quote = %+v", availableQuote)

	return availableQuote.Mul(leverage), nil
}
