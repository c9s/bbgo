package xmaker

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type SpreadMaker struct {
	Enabled bool `json:"enabled"`

	MinProfitRatio   fixedpoint.Value `json:"minProfitRatio"`
	MaxQuoteAmount   fixedpoint.Value `json:"maxQuoteAmount"`
	MaxOrderLifespan types.Duration   `json:"maxOrderLifespan"`

	SignalThreshold float64 `json:"signalThreshold"`

	ReverseSignalOrderCancel bool `json:"reverseSignalOrderCancel"`

	MakerOnly bool `json:"makerOnly"`

	// order is the current spread maker order on the maker exchange
	order *types.Order

	// orderStore stores the history maker orders
	orderStore *core.OrderStore

	session *bbgo.ExchangeSession

	market types.Market

	orderQueryService types.ExchangeOrderQueryService

	symbol string

	mu sync.Mutex
}

func (c *SpreadMaker) Defaults() error {
	if c.MinProfitRatio.IsZero() {
		c.MinProfitRatio = fixedpoint.NewFromFloat(0.01 * 0.01)
	}

	if c.MaxQuoteAmount.IsZero() {
		c.MaxQuoteAmount = fixedpoint.NewFromFloat(100)
	}

	if c.MaxOrderLifespan == 0 {
		c.MaxOrderLifespan = types.Duration(2 * time.Second)
	}

	return nil
}

func (c *SpreadMaker) updateOrder(ctx context.Context) (*types.Order, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	retOrder, err := c.orderQueryService.QueryOrder(ctx, c.order.AsQuery())
	if err != nil {
		return nil, err
	}

	c.order = retOrder
	return retOrder, nil
}

func (c *SpreadMaker) canSpreadMaking(
	signal float64, position *types.Position,
	makerMarket types.Market,
	bestBidPrice, bestAskPrice fixedpoint.Value, // maker best bid price
) (*types.SubmitOrder, bool) {
	side := position.Side()

	if !isSignalSidePosition(signal, side) {
		return nil, false
	}

	if math.Abs(signal) < c.SignalThreshold {
		return nil, false
	}

	base := position.GetBase()
	cost := position.GetAverageCost()
	profitPrice := cost
	if c.MinProfitRatio.Sign() > 0 {
		profitPrice = getPositionProfitPrice(side, profitPrice, c.MinProfitRatio)
	}

	maxQuantity := c.MaxQuoteAmount.Div(cost)
	orderQuantity := base.Abs()
	orderQuantity = fixedpoint.Min(orderQuantity, maxQuantity)
	orderSide := side.Reverse()

	switch orderSide {
	case types.SideTypeSell:
		targetPrice := bestBidPrice.Add(makerMarket.TickSize)
		targetPrice = fixedpoint.Max(profitPrice, targetPrice)
		return c.newMakerOrder(makerMarket, orderSide, targetPrice, orderQuantity), true

	case types.SideTypeBuy:
		targetPrice := bestAskPrice.Sub(makerMarket.TickSize)
		targetPrice = fixedpoint.Min(profitPrice, targetPrice)
		return c.newMakerOrder(makerMarket, orderSide, targetPrice, orderQuantity), true
	}

	return nil, false
}

func (c *SpreadMaker) newMakerOrder(
	market types.Market,
	side types.SideType,
	targetPrice, orderQuantity fixedpoint.Value,
) *types.SubmitOrder {
	orderType := types.OrderTypeLimit
	if c.MakerOnly {
		orderType = types.OrderTypeLimitMaker
	}

	return &types.SubmitOrder{
		// ClientOrderID:    "",
		Symbol:      c.symbol,
		Side:        side,
		Type:        orderType,
		Price:       targetPrice,
		Quantity:    orderQuantity,
		Market:      market,
		TimeInForce: types.TimeInForceGTC,
	}
}

func (c *SpreadMaker) getOrder() (o types.Order, ok bool) {
	c.mu.Lock()
	if c.order != nil {
		o = *c.order
		ok = true
	}
	c.mu.Unlock()
	return o, ok
}

func (c *SpreadMaker) cancelOrder(ctx context.Context) error {
	if order, ok := c.getOrder(); ok {
		return retry.CancelOrdersUntilSuccessful(ctx, c.session.Exchange, order)
	}

	return nil
}

// cancelAndQueryOrder cancels the current order and queries the order status until the order is canceled
func (c *SpreadMaker) cancelAndQueryOrder(ctx context.Context) (*types.Order, error) {
	if c.order == nil {
		return nil, nil
	}

	if err := c.cancelOrder(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	order := c.order
	c.order = nil
	c.mu.Unlock()

	finalOrder, err := retry.QueryOrderUntilCanceled(ctx, c.orderQueryService, order.Symbol, order.OrderID)
	if err != nil {
		return nil, err
	}

	return finalOrder, nil
}

func (c *SpreadMaker) shouldKeepOrder(o types.Order, now time.Time) bool {
	creationTime := o.CreationTime.Time()
	if creationTime.IsZero() {
		return false
	}

	if creationTime.Add(c.MaxOrderLifespan.Duration()).Before(now) {
		return true
	}

	return false
}

func (c *SpreadMaker) placeOrder(ctx context.Context, submitOrder *types.SubmitOrder) (*types.Order, error) {
	createdOrder, err := c.session.Exchange.SubmitOrder(ctx, *submitOrder)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.order = createdOrder
	c.mu.Unlock()
	return createdOrder, nil
}

func (c *SpreadMaker) Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error {
	c.symbol = symbol
	c.orderStore = core.NewOrderStore(symbol)
	c.session = session
	c.market, _ = c.session.Market(symbol)
	c.orderQueryService = c.session.Exchange.(types.ExchangeOrderQueryService)
	return nil
}
