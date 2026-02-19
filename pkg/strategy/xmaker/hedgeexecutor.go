package xmaker

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeExecutor interface {
	// Hedge executes a hedge order based on the uncovered position and the hedge delta
	// uncoveredPosition: the current uncovered position that needs to be hedged
	// hedgeDelta: the delta that needs to be hedged, which is the negative of uncoveredPosition
	// quantity: the absolute value of hedgeDelta, which is the order quantity to be hedged
	// side: the side of the hedge order, which is determined by the sign of hedgeDelta
	Hedge(
		ctx context.Context,
		uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
		side types.SideType,
	) error

	// Clear clears any pending orders or state related to hedging
	Clear(ctx context.Context) error
}

type BaseHedgeExecutorConfig struct {
}

type MarketOrderHedgeExecutorConfig struct {
	BaseHedgeExecutorConfig

	MaxOrderQuantity fixedpoint.Value `json:"maxOrderQuantity,omitempty"` // max order quantity for market order hedge
}

type MarketOrderHedgeExecutor struct {
	*HedgeMarket

	config *MarketOrderHedgeExecutorConfig
}

func NewMarketOrderHedgeExecutor(
	market *HedgeMarket,
	config *MarketOrderHedgeExecutorConfig,
) *MarketOrderHedgeExecutor {
	return &MarketOrderHedgeExecutor{
		HedgeMarket: market,
		config:      config,
	}
}

func (m *MarketOrderHedgeExecutor) Clear(ctx context.Context) error {
	// no-op for market order hedge executor
	return nil
}

func (m *MarketOrderHedgeExecutor) Hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	bid, ask := m.GetQuotePrice()
	price := sideTakerPrice(bid, ask, side)

	if !m.session.Margin {
		quantity = AdjustHedgeQuantityWithAvailableBalance(
			m.session.GetAccount(), m.market, side, quantity, price,
		)
	}

	if m.config != nil {
		if m.config.MaxOrderQuantity.Sign() > 0 {
			quantity = fixedpoint.Min(quantity, m.config.MaxOrderQuantity)
		}
	}

	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("MarketOrderHedgeExecutor: skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	marginSideEffect := types.SideEffectTypeMarginBuy
	account := m.session.GetAccount()
	if !account.MarginLevel.IsZero() && !m.MinMarginLevel.IsZero() && account.MarginLevel.Compare(m.MinMarginLevel) < 0 {
		marginSideEffect = types.SideEffectTypeAutoRepay
	}

	orderForm := types.SubmitOrder{
		Market:           m.market,
		Symbol:           m.market.Symbol,
		Side:             side,
		Type:             types.OrderTypeMarket,
		Quantity:         quantity,
		MarginSideEffect: marginSideEffect,
	}

	m.logger.Infof("MarketOrderHedgeExecutor: submitting hedge order: %+v", orderForm)

	// for positive +3 position
	// hedge delta = -3
	// hedge delta = -3 means we need to sell 3 to hedge
	// order side = sell
	// to cover position, cover delta = -(hedge delta) = -(-3) = +3
	// to uncover the +3 position, we need to send -3 position to cover. (uncover = -cover)
	coverDelta := quantityToDelta(quantity, side).Neg()
	m.positionExposure.Cover(coverDelta)

	hedgeOrder, err := m.submitOrder(ctx, orderForm)
	if err != nil {
		// if error occurs, revert the covered position
		if dispatchErr := m.HedgeMarket.RedispatchCoveredPosition(coverDelta); dispatchErr != nil {
			m.logger.WithError(dispatchErr).
				Errorf("failed to redispatch position after hedge order failure: delta=%s", coverDelta.String())
		}

		return err
	}

	m.logger.Infof("hedge order created: %+v", hedgeOrder)

	if hedgeOrder != nil {
		updatedOrder, err := m.syncOrder(ctx, *hedgeOrder)
		if err != nil {
			m.logger.WithError(err).WithFields(hedgeOrder.LogFields()).Errorf("failed to sync order: %+v", hedgeOrder)
		} else if updatedOrder != nil {
			orderLogger := m.logger.WithFields(updatedOrder.LogFields())
			// Compare the order quantity, if the order quantity is changed from the server side
			// we should uncover the difference
			if !updatedOrder.Quantity.Eq(hedgeOrder.Quantity) {
				// if we sent sell 1 BTC (-1 BTC), but the order was adjusted to sell 0.1 BTC (-0.1 BTC)
				// we should uncover +0.9 BTC to the position exposure (meaning -0.9 BTC the the covered position)

				// Net Position: 1.0, Pending: 0.0
				// Submit Order { Quantity: 1, Side: Sell }
				// Net Position: 1.0, Pending: 1.0
				// Order Updated { Quantity: 0.1, Side: Sell }
				// to quantityToDelta = -(-0.9) (Sell) = +0.9
				// to coverDeltaDiff = +0.9
				// to uncover = 0.9 = cover -0.9
				// Net Position: 1.0, Pending: 0.1
				// Covered Position: 0.1
				// Uncover: 0.9
				quantityDiff := updatedOrder.Quantity.Sub(hedgeOrder.Quantity)

				orderLogger.Infof("hedge order quantity changed from %s to %s, adjusting covered position by %s",
					hedgeOrder.Quantity.String(), updatedOrder.Quantity.String(), quantityDiff.String())

				if !quantityDiff.IsZero() {
					coverDeltaDiff := quantityToDelta(quantityDiff, side)
					m.positionExposure.Uncover(coverDeltaDiff)
					orderLogger.Infof("hedge order quantity changed from %s to %s, adjusting covered position by %s",
						hedgeOrder.Quantity.String(), updatedOrder.Quantity.String(), coverDeltaDiff.String())
				}
			}
		}
	}

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

func (m *CounterpartyHedgeExecutor) Clear(ctx context.Context) error {
	if m.hedgeOrder == nil {
		return nil
	}

	if o, ok := m.HedgeMarket.activeMakerOrders.Get(m.hedgeOrder.OrderID); ok {
		m.hedgeOrder = &o
	}

	// trigger CancelOrders only if the order is not already closed
	switch m.hedgeOrder.Status {
	case types.OrderStatusNew, types.OrderStatusPartiallyFilled:
		m.logger.Infof("cancelling existing hedge order: %+v", m.hedgeOrder)
		if err := m.session.Exchange.CancelOrders(ctx, *m.hedgeOrder); err != nil {
			m.logger.WithError(err).Errorf("failed to cancel order: %+v", m.hedgeOrder)
		}
	}

	hedgeOrder, err := retry.QueryOrderUntilCanceled(ctx, m.session.Exchange.(types.ExchangeOrderQueryService), m.hedgeOrder.AsQuery())
	if err != nil {
		m.logger.WithError(err).Errorf("failed to query order after cancel: %+v", m.hedgeOrder)
	} else {

		// return covered position from the canceled order
		coverDelta := quantityToDelta(hedgeOrder.GetRemainingQuantity(), hedgeOrder.Side).Neg()
		m.logger.Infof("hedge order is %s: %+v, returning covered position %s", hedgeOrder.Status, hedgeOrder, coverDelta.String())
		m.positionExposure.Uncover(coverDelta)
	}

	m.hedgeOrder = nil
	return err
}

func (m *CounterpartyHedgeExecutor) Hedge(
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

	coverDelta := quantityToDelta(quantity, side).Neg()
	m.positionExposure.Cover(coverDelta)

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol:   m.market.Symbol,
		Market:   m.market,
		Type:     types.OrderTypeLimit,
		Side:     side,
		Price:    price,
		Quantity: quantity,
	})

	if err != nil {
		m.positionExposure.Uncover(coverDelta)
		return err
	}

	m.hedgeOrder = hedgeOrder
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
