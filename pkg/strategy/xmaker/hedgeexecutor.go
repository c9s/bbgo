package xmaker

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeExecutor interface {
	// hedge executes a hedge order based on the uncovered position and the hedge delta
	// uncoveredPosition: the current uncovered position that needs to be hedged
	// hedgeDelta: the delta that needs to be hedged, which is the negative of uncoveredPosition
	// quantity: the absolute value of hedgeDelta, which is the order quantity to be hedged
	// side: the side of the hedge order, which is determined by the sign of hedgeDelta
	hedge(
		ctx context.Context,
		uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
		side types.SideType,
	) error

	canHedge(
		ctx context.Context,
		uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
		side types.SideType,
	) (bool, error)

	// clear clears any pending orders or state related to hedging
	clear(ctx context.Context) error
}

type BaseHedgeExecutorConfig struct {
}

type MarketOrderHedgeExecutorConfig struct {
	HedgeExecutor

	BaseHedgeExecutorConfig

	MaxOrderQuantity fixedpoint.Value `json:"maxOrderQuantity,omitempty"` // max order quantity for market order hedge
}

type MarketOrderHedgeExecutor struct {
	*HedgeMarket

	config *MarketOrderHedgeExecutorConfig
}

func newMarketOrderHedgeExecutor(
	market *HedgeMarket,
	config *MarketOrderHedgeExecutorConfig,
) *MarketOrderHedgeExecutor {
	return &MarketOrderHedgeExecutor{
		HedgeMarket: market,
		config:      config,
	}
}

func (m *MarketOrderHedgeExecutor) clear(ctx context.Context) error {
	// no-op for market order hedge executor
	return nil
}

func (m *MarketOrderHedgeExecutor) canHedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) (bool, error) {
	// get quote price
	bid, ask := m.getQuotePrice()
	price := sideTakerPrice(bid, ask, side)
	currency, required := determineRequiredCurrencyAndAmount(m.market, side, quantity, price)
	account := m.session.GetAccount()
	available, ok := getAvailableBalance(account, currency)
	if !ok {
		log.Warnf("cannot find balance for currency: %s", currency)
		return false, nil
	}

	// for margin account, we need to check if the margin level is sufficient
	if m.session.Margin {
		// a simple check to ensure the account is not in danger of liquidation
		if account.MarginLevel.IsZero() || account.MarginLevel.Compare(fixedpoint.NewFromFloat(2.0)) < 0 {
			log.Warnf("margin level too low to hedge: %s", account.MarginLevel.String())
			return false, nil
		}
	}

	if !isBalanceSufficient(available, required) {
		log.Warnf("insufficient balance for hedge: need %s %s, available %s", required.String(), currency, available.String())
		return false, nil
	}

	if m.market.IsDustQuantity(quantity, price) {
		log.Warnf("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return false, nil
	}

	return true, nil
}

func (m *MarketOrderHedgeExecutor) hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	bid, ask := m.getQuotePrice()
	price := sideTakerPrice(bid, ask, side)

	quantity = AdjustHedgeQuantityWithAvailableBalance(
		m.session.GetAccount(), m.market, side, quantity, price,
	)

	if m.config != nil {
		if m.config.MaxOrderQuantity.Sign() > 0 {
			quantity = fixedpoint.Min(quantity, m.config.MaxOrderQuantity)
		}
	}

	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol:           m.market.Symbol,
		Side:             side,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		Market:           m.market,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	})

	if err != nil {
		return err
	}

	m.positionExposure.Cover(quantity.Mul(toSign(uncoveredPosition)))

	m.logger.Infof("hedge order created: %+v", hedgeOrder)
	return nil
}

type CounterpartyHedgeExecutorConfig struct {
	BaseHedgeExecutorConfig

	PriceLevel int `json:"priceLevel"`
}

type CounterpartyHedgeExecutor struct {
	HedgeExecutor

	*HedgeMarket

	config     *CounterpartyHedgeExecutorConfig
	hedgeOrder *types.Order
}

func newCounterpartyHedgeExecutor(
	market *HedgeMarket,
	config *CounterpartyHedgeExecutorConfig,
) *CounterpartyHedgeExecutor {
	return &CounterpartyHedgeExecutor{
		HedgeMarket: market,
		config:      config,
	}
}

func (m *CounterpartyHedgeExecutor) canHedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) (bool, error) {
	// TODO: implement this
	return true, nil
}

func (m *CounterpartyHedgeExecutor) clear(ctx context.Context) error {
	if m.hedgeOrder == nil {
		return nil
	}

	if err := m.session.Exchange.CancelOrders(ctx, *m.hedgeOrder); err != nil {
		m.logger.WithError(err).Errorf("failed to cancel order: %+v", m.hedgeOrder)
	}

	hedgeOrder, err := retry.QueryOrderUntilCanceled(ctx, m.session.Exchange.(types.ExchangeOrderQueryService), m.hedgeOrder.AsQuery())
	if err != nil {
		m.logger.WithError(err).Errorf("failed to query order after cancel: %+v", m.hedgeOrder)
	} else {
		m.logger.Infof("hedge order canceled: %+v, returning covered position...", hedgeOrder)

		// return covered position from the canceled order
		delta := quantityToDelta(hedgeOrder.GetRemainingQuantity(), hedgeOrder.Side)
		if !delta.IsZero() {
			m.positionExposure.Cover(delta)
		}
	}

	m.hedgeOrder = nil
	return err
}

func (m *CounterpartyHedgeExecutor) hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	// use counterparty side book
	counterpartySide := side.Reverse()
	sideBook := m.book.SideBook(counterpartySide)

	if len(sideBook) == 0 {
		return fmt.Errorf("side book is empty for %s", m.SymbolSelector)
	}

	priceLevel := m.config.PriceLevel
	offset := 0
	if m.config.PriceLevel < 0 {
		offset = m.config.PriceLevel
		priceLevel = 0
	} else {
		priceLevel--
	}

	if priceLevel > len(sideBook) {
		return fmt.Errorf("invalid price level %d for %s", m.config.PriceLevel, m.SymbolSelector)
	}

	price := sideBook[priceLevel].Price
	if offset > 0 {
		ticks := m.market.TickSize.Mul(fixedpoint.NewFromInt(int64(offset)))
		switch counterpartySide {
		case types.SideTypeBuy:
			price = price.Add(ticks)
		case types.SideTypeSell:
			price = price.Sub(ticks)
		}
	}

	quantity = AdjustHedgeQuantityWithAvailableBalance(
		m.session.GetAccount(), m.market, side, quantity, price,
	)

	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol:   m.market.Symbol,
		Market:   m.market,
		Type:     types.OrderTypeLimit,
		Side:     side,
		Price:    price,
		Quantity: quantity,
	})

	if err != nil {
		return err
	}

	m.hedgeOrder = hedgeOrder
	m.positionExposure.Cover(quantity.Mul(toSign(uncoveredPosition)))
	m.logger.Infof("hedge order created: %+v", hedgeOrder)
	return nil
}

func toSign(v fixedpoint.Value) fixedpoint.Value {
	if v.Sign() < 0 {
		return fixedpoint.NegOne
	}

	return fixedpoint.One
}

// determineRequiredCurrencyAndAmount returns the required currency and amount for hedging
func determineRequiredCurrencyAndAmount(
	market types.Market, side types.SideType, quantity, price fixedpoint.Value,
) (string, fixedpoint.Value) {
	if side == types.SideTypeBuy {
		return market.QuoteCurrency, quantity.Mul(price)
	}
	return market.BaseCurrency, quantity
}

// getAvailableBalance returns the available balance for the given currency
func getAvailableBalance(account *types.Account, currency string) (fixedpoint.Value, bool) {
	balance, ok := account.Balance(currency)
	if !ok {
		return fixedpoint.Zero, false
	}
	return balance.Available, true
}

// isBalanceSufficient checks if available balance is sufficient for required amount
func isBalanceSufficient(available, required fixedpoint.Value) bool {
	return available.Compare(required) >= 0
}
