package bbgo

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/risk"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultLeverage = fixedpoint.NewFromInt(3)

var maxIsolatedMarginLeverage = fixedpoint.NewFromInt(10)

var maxCrossMarginLeverage = fixedpoint.NewFromInt(3)

type AccountValueCalculator struct {
	priceSolver   *pricesolver.SimplePriceSolver
	session       *ExchangeSession
	quoteCurrency string
}

func NewAccountValueCalculator(
	session *ExchangeSession,
	priceSolver *pricesolver.SimplePriceSolver,
	quoteCurrency string,
) *AccountValueCalculator {
	return &AccountValueCalculator{
		priceSolver:   priceSolver,
		session:       session,
		quoteCurrency: quoteCurrency,
	}
}

// UpdatePrices updates the price index from the existing balances
func (c *AccountValueCalculator) UpdatePrices(ctx context.Context) error {
	balances := c.session.Account.Balances()
	currencies := balances.Currencies()
	markets := c.session.Markets()

	var symbols []string
	for _, currency := range currencies {
		if currency == c.quoteCurrency {
			continue
		}

		symbol := currency + c.quoteCurrency
		reversedSymbol := c.quoteCurrency + currency
		if _, ok := markets[symbol]; ok {
			symbols = append(symbols, symbol)
		} else if _, ok2 := markets[reversedSymbol]; ok2 {
			symbols = append(symbols, reversedSymbol)
		}
	}

	return c.priceSolver.UpdateFromTickers(ctx, c.session.Exchange, symbols...)
}

func (c *AccountValueCalculator) DebtValue() fixedpoint.Value {
	balances := c.session.Account.Balances()
	return totalValueInQuote(balances, c.priceSolver, c.quoteCurrency, func(
		prev fixedpoint.Value, b types.Balance, price fixedpoint.Value,
	) fixedpoint.Value {
		return prev.Add(b.Debt().Mul(price))
	})
}

func (c *AccountValueCalculator) MarketValue() fixedpoint.Value {
	balances := c.session.Account.Balances()
	return totalValueInQuote(balances, c.priceSolver, c.quoteCurrency, func(
		prev fixedpoint.Value, b types.Balance, price fixedpoint.Value,
	) fixedpoint.Value {
		return prev.Add(b.Total().Mul(price))
	})

}

func (c *AccountValueCalculator) NetValue() fixedpoint.Value {
	balances := c.session.Account.Balances()
	return totalValueInQuote(balances, c.priceSolver, c.quoteCurrency, func(
		prev fixedpoint.Value, b types.Balance, price fixedpoint.Value,
	) fixedpoint.Value {
		return prev.Add(b.Net().Mul(price))
	})
}

func totalValueInQuote(
	balances types.BalanceMap,
	priceSolver *pricesolver.SimplePriceSolver,
	quoteCurrency string,
	algo func(prev fixedpoint.Value, b types.Balance, price fixedpoint.Value) fixedpoint.Value,
) (totalValue fixedpoint.Value) {
	totalValue = fixedpoint.Zero

	for _, b := range balances {
		if b.Currency == quoteCurrency {
			totalValue = algo(totalValue, b, fixedpoint.One)
			continue
		} else if price, ok := priceSolver.ResolvePrice(b.Currency, quoteCurrency); ok {
			totalValue = algo(totalValue, b, price)
		}
	}

	return totalValue
}

func calculateNetValueInQuote(
	balances types.BalanceMap,
	priceSolver *pricesolver.SimplePriceSolver,
	quoteCurrency string,
) fixedpoint.Value {
	return totalValueInQuote(balances, priceSolver, quoteCurrency, func(
		prev fixedpoint.Value, b types.Balance, price fixedpoint.Value,
	) fixedpoint.Value {
		return prev.Add(b.Net().Mul(price))
	})
}

func (c *AccountValueCalculator) AvailableQuote() (fixedpoint.Value, error) {
	accountValue := fixedpoint.Zero

	balances := c.session.Account.Balances()
	for _, b := range balances {
		if b.Currency == c.quoteCurrency {
			accountValue = accountValue.Add(b.Net())
			continue
		}

		if price, ok := c.priceSolver.ResolvePrice(b.Currency, c.quoteCurrency); ok {
			accountValue = accountValue.Add(b.Net().Mul(price))
		}
	}

	return accountValue, nil
}

// MarginLevel calculates the margin level from the asset market value and the debt value
// See https://www.binance.com/en/support/faq/360030493931
func (c *AccountValueCalculator) MarginLevel() (fixedpoint.Value, error) {
	marketValue := c.MarketValue()
	debtValue := c.DebtValue()

	if marketValue.IsZero() || debtValue.IsZero() {
		return fixedpoint.NewFromFloat(999.0), nil
	}

	return marketValue.Div(debtValue), nil
}

func aggregateUsdNetValue(balances types.BalanceMap) fixedpoint.Value {
	totalUsdValue := fixedpoint.Zero
	// get all usd value if any
	for currency, balance := range balances {
		if types.IsUSDFiatCurrency(currency) {
			totalUsdValue = totalUsdValue.Add(balance.Net())
		}
	}

	return totalUsdValue
}

func usdFiatBalances(balances types.BalanceMap) (fiats types.BalanceMap, rest types.BalanceMap) {
	rest = make(types.BalanceMap)
	fiats = make(types.BalanceMap)
	for currency, balance := range balances {
		if types.IsUSDFiatCurrency(currency) {
			fiats[currency] = balance
		} else {
			rest[currency] = balance
		}
	}

	return fiats, rest
}

func CalculateBaseQuantity(
	session *ExchangeSession, market types.Market, price, quantity, leverage fixedpoint.Value,
) (fixedpoint.Value, error) {
	// default leverage guard
	if leverage.IsZero() {
		leverage = defaultLeverage
	}

	baseBalance, hasBaseBalance := session.Account.Balance(market.BaseCurrency)
	balances := session.Account.Balances()

	usingLeverage := session.Margin || session.IsolatedMargin || session.Futures || session.IsolatedFutures
	if !usingLeverage {
		// For spot, we simply sell the base quoteCurrency
		if hasBaseBalance {
			if quantity.IsZero() {
				log.Warnf("sell quantity is not set, using all available base balance: %v", baseBalance)
				if !baseBalance.Available.IsZero() {
					return baseBalance.Available, nil
				}
			} else {
				return fixedpoint.Min(quantity, baseBalance.Available), nil
			}
		}

		return quantity, types.NewZeroAssetError(
			fmt.Errorf("quantity is zero, can not submit sell order, please check your quantity settings, your account balances: %+v", balances))
	}

	usdBalances, restBalances := usdFiatBalances(balances)

	// for isolated margin, we can calculate from these two pair
	totalUsdValue := fixedpoint.Zero
	if len(restBalances) == 1 && types.IsUSDFiatCurrency(market.QuoteCurrency) {
		totalUsdValue = aggregateUsdNetValue(balances)
	} else if len(restBalances) > 1 {
		priceSolver := pricesolver.NewSimplePriceResolver(session.Markets())
		accountValue := NewAccountValueCalculator(session, priceSolver, "USDT")
		if err := accountValue.UpdatePrices(context.Background()); err != nil {
			return fixedpoint.Zero, err
		}

		netValue := accountValue.NetValue()
		totalUsdValue = netValue
	} else {
		// TODO: translate quote currency like BTC of ETH/BTC to usd value
		totalUsdValue = aggregateUsdNetValue(usdBalances)
	}

	if !quantity.IsZero() {
		return quantity, nil
	}

	if price.IsZero() {
		return quantity, fmt.Errorf("%s price can not be zero", market.Symbol)
	}

	// using leverage -- starts from here
	log.Infof("calculating available leveraged base quantity: base balance = %+v, total usd value %f", baseBalance, totalUsdValue.Float64())

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
		maxPositionQuantity := risk.CalculateMaxPosition(price, totalUsdValue, leverage)

		return maxPositionQuantity, nil
	}

	return quantity, types.NewZeroAssetError(
		errors.New("quantity is zero, can not submit sell order, please check your settings"))
}

func CalculateQuoteQuantity(
	ctx context.Context, session *ExchangeSession, quoteCurrency string, leverage fixedpoint.Value,
) (fixedpoint.Value, error) {
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
	priceSolver := pricesolver.NewSimplePriceResolver(session.Markets())

	accountValue := NewAccountValueCalculator(session, priceSolver, quoteCurrency)
	if err := accountValue.UpdatePrices(ctx); err != nil {
		return fixedpoint.Zero, err
	}

	availableQuote, err := accountValue.AvailableQuote()
	if err != nil {
		log.WithError(err).Errorf("can not update available quote")
		return fixedpoint.Zero, err
	}

	log.Infof("calculating available leveraged quote quantity: account available quote = %+v", availableQuote)

	return availableQuote.Mul(leverage), nil
}
