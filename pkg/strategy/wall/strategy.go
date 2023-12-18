package wall

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/strategy/common"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "wall"

const stateKey = "state-v1"

var defaultFeeRate = fixedpoint.NewFromFloat(0.001)
var two = fixedpoint.NewFromInt(2)

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	// Symbol is the market symbol you want to trade
	Symbol string `json:"symbol"`

	Side types.SideType `json:"side"`

	// Interval is how long do you want to update your order price and quantity
	Interval types.Interval `json:"interval"`

	FixedPrice fixedpoint.Value `json:"fixedPrice"`

	bbgo.QuantityOrAmount

	NumLayers int `json:"numLayers"`

	// LayerSpread is the price spread between each layer
	LayerSpread fixedpoint.Value `json:"layerSpread"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	AdjustmentMinSpread fixedpoint.Value `json:"adjustmentMinSpread"`
	AdjustmentQuantity  fixedpoint.Value `json:"adjustmentQuantity"`

	session *bbgo.ExchangeSession

	activeAdjustmentOrders *bbgo.ActiveOrderBook
	activeWallOrders       *bbgo.ActiveOrderBook
}

func (s *Strategy) Initialize() error {
	s.Strategy = &common.Strategy{}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{
		Depth: types.DepthLevelFull,
	})
}

func (s *Strategy) Validate() error {
	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	if len(s.Side) == 0 {
		return errors.New("side is required")
	}

	if s.FixedPrice.IsZero() {
		return errors.New("fixedPrice can not be zero")
	}

	return nil
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) placeAdjustmentOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor) error {
	var submitOrders []types.SubmitOrder
	// position adjustment orders
	base := s.Position.GetBase()
	if base.IsZero() {
		return nil
	}

	ticker, err := s.session.Exchange.QueryTicker(ctx, s.Symbol)
	if err != nil {
		return err
	}

	if s.Market.IsDustQuantity(base, ticker.Last) {
		return nil
	}

	switch s.Side {
	case types.SideTypeBuy:
		askPrice := ticker.Sell.Mul(s.AdjustmentMinSpread.Add(fixedpoint.One))

		if s.Position.AverageCost.Compare(askPrice) <= 0 {
			return nil
		}

		if base.Sign() < 0 {
			return nil
		}

		quantity := base.Abs()
		if quantity.Compare(s.AdjustmentQuantity) >= 0 {
			quantity = s.AdjustmentQuantity
		}

		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     s.Side.Reverse(),
			Type:     types.OrderTypeLimitMaker,
			Price:    askPrice,
			Quantity: quantity,
			Market:   s.Market,
		})

	case types.SideTypeSell:
		bidPrice := ticker.Sell.Mul(fixedpoint.One.Sub(s.AdjustmentMinSpread))

		if s.Position.AverageCost.Compare(bidPrice) >= 0 {
			return nil
		}

		if base.Sign() > 0 {
			return nil
		}

		quantity := base.Abs()
		if quantity.Compare(s.AdjustmentQuantity) >= 0 {
			quantity = s.AdjustmentQuantity
		}

		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     s.Side.Reverse(),
			Type:     types.OrderTypeLimitMaker,
			Price:    bidPrice,
			Quantity: quantity,
			Market:   s.Market,
		})
	}

	// condition for lower the average cost
	if len(submitOrders) == 0 {
		return nil
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		return err
	}

	s.activeAdjustmentOrders.Add(createdOrders...)
	return nil
}

func (s *Strategy) placeWallOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor) error {
	log.Infof("placing wall orders...")

	var submitOrders []types.SubmitOrder
	var startPrice = s.FixedPrice
	for i := 0; i < s.NumLayers; i++ {
		var price = startPrice
		var quantity fixedpoint.Value
		if s.QuantityOrAmount.IsSet() {
			quantity = s.QuantityOrAmount.CalculateQuantity(price)
		} else if s.QuantityScale != nil {
			qf, err := s.QuantityScale.Scale(i + 1)
			if err != nil {
				return err
			}
			quantity = fixedpoint.NewFromFloat(qf)
		}

		order := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     s.Side,
			Type:     types.OrderTypeLimitMaker,
			Price:    price,
			Quantity: quantity,
			Market:   s.Market,
		}
		submitOrders = append(submitOrders, order)
		switch s.Side {
		case types.SideTypeSell:
			startPrice = startPrice.Add(s.LayerSpread)

		case types.SideTypeBuy:
			startPrice = startPrice.Sub(s.LayerSpread)

		}
	}

	// condition for lower the average cost
	if len(submitOrders) == 0 {
		return nil
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, submitOrders...)
	if err != nil {
		return err
	}

	log.Infof("wall orders placed: %+v", createdOrders)

	s.activeWallOrders.Add(createdOrders...)
	return err
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	// initial required information
	s.session = session

	s.activeWallOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeWallOrders.BindStream(session.UserDataStream)

	s.activeAdjustmentOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeAdjustmentOrders.BindStream(session.UserDataStream)

	session.UserDataStream.OnStart(func() {
		if err := s.placeWallOrders(ctx, s.OrderExecutor); err != nil {
			log.WithError(err).Errorf("can not place order")
		}
	})

	s.activeAdjustmentOrders.OnFilled(func(o types.Order) {
		if err := s.activeAdjustmentOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// check if there is a canceled order had partially filled.
		s.OrderExecutor.TradeCollector().Process()

		if err := s.placeAdjustmentOrders(ctx, s.OrderExecutor); err != nil {
			log.WithError(err).Errorf("can not place order")
		}
	})

	s.activeWallOrders.OnFilled(func(o types.Order) {
		if err := s.activeWallOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// check if there is a canceled order had partially filled.
		s.OrderExecutor.TradeCollector().Process()

		if err := s.placeWallOrders(ctx, s.OrderExecutor); err != nil {
			log.WithError(err).Errorf("can not place order")
		}

		if err := s.activeAdjustmentOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// check if there is a canceled order had partially filled.
		s.OrderExecutor.TradeCollector().Process()

		if err := s.placeAdjustmentOrders(ctx, s.OrderExecutor); err != nil {
			log.WithError(err).Errorf("can not place order")
		}
	})

	ticker := time.NewTicker(s.Interval.Duration())
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				orders := s.activeWallOrders.Orders()
				if anyOrderFilled(orders) {
					if err := s.activeWallOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
						log.WithError(err).Errorf("graceful cancel order error")
					}

					// check if there is a canceled order had partially filled.
					s.OrderExecutor.TradeCollector().Process()

					if err := s.placeWallOrders(ctx, s.OrderExecutor); err != nil {
						log.WithError(err).Errorf("can not place order")
					}
				}
			}
		}
	}()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if err := s.activeWallOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		if err := s.activeAdjustmentOrders.GracefulCancel(ctx, s.session.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel order error")
		}

		// check if there is a canceled order had partially filled.
		s.OrderExecutor.TradeCollector().Process()
	})

	return nil
}

func anyOrderFilled(orders []types.Order) bool {
	for _, o := range orders {
		if o.ExecutedQuantity.Sign() > 0 {
			return true
		}
	}
	return false
}
