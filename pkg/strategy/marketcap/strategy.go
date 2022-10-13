package marketcap

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datasource/coinmarketcap"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "marketcap"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	datasource *coinmarketcap.DataSource

	// interval to rebalance the portfolio
	Interval            types.Interval   `json:"interval"`
	QuoteCurrency       string           `json:"quoteCurrency"`
	QuoteCurrencyWeight fixedpoint.Value `json:"quoteCurrencyWeight"`
	BaseCurrencies      []string         `json:"baseCurrencies"`
	Threshold           fixedpoint.Value `json:"threshold"`
	DryRun              bool             `json:"dryRun"`
	// max amount to buy or sell per order
	MaxAmount fixedpoint.Value `json:"maxAmount"`
	// interval to query marketcap data from coinmarketcap
	QueryInterval types.Interval `json:"queryInterval"`

	subscribeSymbol string
	activeOrderBook *bbgo.ActiveOrderBook
	targetWeights   types.ValueMap
}

func (s *Strategy) Initialize() error {
	apiKey := os.Getenv("COINMARKETCAP_API_KEY")
	s.datasource = coinmarketcap.New(apiKey)

	// select one symbol to subscribe
	s.subscribeSymbol = s.BaseCurrencies[0] + s.QuoteCurrency

	s.activeOrderBook = bbgo.NewActiveOrderBook("")
	s.targetWeights = types.ValueMap{}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if len(s.BaseCurrencies) == 0 {
		return fmt.Errorf("taretCurrencies should not be empty")
	}

	for _, c := range s.BaseCurrencies {
		if c == s.QuoteCurrency {
			return fmt.Errorf("targetCurrencies contain baseCurrency")
		}
	}

	if s.Threshold.Sign() < 0 {
		return fmt.Errorf("threshold should not less than 0")
	}

	if s.MaxAmount.Sign() < 0 {
		return fmt.Errorf("maxAmount shoud not less than 0")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	symbol := s.BaseCurrencies[0] + s.QuoteCurrency
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, symbol, types.SubscribeOptions{Interval: s.QueryInterval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.activeOrderBook.BindStream(session.UserDataStream)

	s.updateTargetWeights(ctx)
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if kline.Interval == s.QueryInterval {
			s.updateTargetWeights(ctx)
		}

		if kline.Interval == s.Interval {
			s.rebalance(ctx, orderExecutor, session)
		}
	})
	return nil
}

func (s *Strategy) rebalance(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) {
	if err := orderExecutor.CancelOrders(ctx, s.activeOrderBook.Orders()...); err != nil {
		log.WithError(err).Error("failed to cancel orders")
	}

	submitOrders := s.generateSubmitOrders(ctx, session)
	for _, submitOrder := range submitOrders {
		log.Infof("generated submit order: %s", submitOrder.String())
	}

	if s.DryRun {
		return
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		log.WithError(err).Error("failed to submit orders")
		return
	}

	s.activeOrderBook.Add(createdOrders...)
}

func (s *Strategy) generateSubmitOrders(ctx context.Context, session *bbgo.ExchangeSession) (submitOrders []types.SubmitOrder) {
	prices := s.prices(ctx, session)
	marketValues := prices.Mul(s.quantities(session))
	currentWeights := marketValues.Normalize()

	for currency, targetWeight := range s.targetWeights {
		if currency == s.QuoteCurrency {
			continue
		}
		symbol := currency + s.QuoteCurrency
		currentWeight := currentWeights[currency]
		currentPrice := prices[currency]

		log.Infof("%s price: %v, current weight: %v, target weight: %v",
			symbol,
			currentPrice,
			currentWeight,
			targetWeight)

		// calculate the difference between current weight and target weight
		// if the difference is less than threshold, then we will not create the order
		weightDifference := targetWeight.Sub(currentWeight)
		if weightDifference.Abs().Compare(s.Threshold) < 0 {
			log.Infof("%s weight distance |%v - %v| = |%v| less than the threshold: %v",
				symbol,
				currentWeight,
				targetWeight,
				weightDifference,
				s.Threshold)
			continue
		}

		quantity := weightDifference.Mul(marketValues.Sum()).Div(currentPrice)

		side := types.SideTypeBuy
		if quantity.Sign() < 0 {
			side = types.SideTypeSell
			quantity = quantity.Abs()
		}

		if s.MaxAmount.Sign() > 0 {
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, currentPrice, s.MaxAmount)
			log.Infof("adjust the quantity %v (%s %s @ %v) by max amount %v",
				quantity,
				symbol,
				side.String(),
				currentPrice,
				s.MaxAmount)
		}

		order := types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     types.OrderTypeLimit,
			Quantity: quantity,
			Price:    currentPrice,
		}

		submitOrders = append(submitOrders, order)
	}
	return submitOrders
}

func (s *Strategy) updateTargetWeights(ctx context.Context) {
	m := floats.Map{}

	// get marketcap from coinmarketcap
	// set higher query limit to avoid target currency not in the list
	marketcaps, err := s.datasource.QueryMarketCapInUSD(ctx, 100)
	if err != nil {
		log.WithError(err).Error("failed to query market cap")
	}

	for _, currency := range s.BaseCurrencies {
		m[currency] = marketcaps[currency]
	}

	// normalize
	m = m.Normalize()

	// rescale by 1 - baseWeight
	m = m.MulScalar(1.0 - s.QuoteCurrencyWeight.Float64())

	// append base weight
	m[s.QuoteCurrency] = s.QuoteCurrencyWeight.Float64()

	// convert to types.ValueMap
	for currency, weight := range m {
		s.targetWeights[currency] = fixedpoint.NewFromFloat(weight)
	}

	log.Infof("target weights: %v", s.targetWeights)
}

func (s *Strategy) prices(ctx context.Context, session *bbgo.ExchangeSession) types.ValueMap {
	tickers, err := session.Exchange.QueryTickers(ctx, s.symbols()...)
	if err != nil {
		log.WithError(err).Error("failed to query tickers")
		return nil
	}

	prices := types.ValueMap{}
	for _, currency := range s.BaseCurrencies {
		prices[currency] = tickers[currency+s.QuoteCurrency].Last
	}

	// append base currency price
	prices[s.QuoteCurrency] = fixedpoint.One

	return prices
}

func (s *Strategy) quantities(session *bbgo.ExchangeSession) types.ValueMap {
	balances := session.Account.Balances()

	quantities := types.ValueMap{}
	for _, currency := range s.currencies() {
		quantities[currency] = balances[currency].Total()
	}

	return quantities
}

func (s *Strategy) symbols() (symbols []string) {
	for _, currency := range s.BaseCurrencies {
		symbols = append(symbols, currency+s.QuoteCurrency)
	}
	return symbols
}

func (s *Strategy) currencies() (currencies []string) {
	currencies = append(currencies, s.BaseCurrencies...)
	currencies = append(currencies, s.QuoteCurrency)
	return currencies
}
