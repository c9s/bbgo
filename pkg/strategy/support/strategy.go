package support

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/service"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "support"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

var zeroiw = types.IntervalWindow{}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position            *types.Position   `json:"position,omitempty"`
	CurrentHighestPrice *fixedpoint.Value `json:"currentHighestPrice,omitempty"`
}

type Target struct {
	ProfitPercentage      fixedpoint.Value                `json:"profitPercentage"`
	QuantityPercentage    fixedpoint.Value                `json:"quantityPercentage"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

// PercentageTargetStop is a kind of stop order by setting fixed percentage target
type PercentageTargetStop struct {
	Targets []Target `json:"targets"`
}

// GenerateOrders generates the orders from the given targets
func (stop *PercentageTargetStop) GenerateOrders(market types.Market, pos *types.Position) []types.SubmitOrder {
	var price = pos.AverageCost
	var quantity = pos.GetBase()

	// submit target orders
	var targetOrders []types.SubmitOrder
	for _, target := range stop.Targets {
		targetPrice := price.Mul(fixedpoint.One.Add(target.ProfitPercentage))
		targetQuantity := quantity.Mul(target.QuantityPercentage)
		targetQuoteQuantity := targetPrice.Mul(targetQuantity)

		if targetQuoteQuantity.Compare(market.MinNotional) <= 0 {
			continue
		}

		if targetQuantity.Compare(market.MinQuantity) <= 0 {
			continue
		}

		targetOrders = append(targetOrders, types.SubmitOrder{
			Symbol:           market.Symbol,
			Market:           market,
			Type:             types.OrderTypeLimit,
			Side:             types.SideTypeSell,
			Price:            targetPrice,
			Quantity:         targetQuantity,
			MarginSideEffect: target.MarginOrderSideEffect,
			TimeInForce:      types.TimeInForceGTC,
		})
	}

	return targetOrders
}

type TrailingStopTarget struct {
	TrailingStopCallbackRatio fixedpoint.Value `json:"callbackRatio"`
	MinimumProfitPercentage   fixedpoint.Value `json:"minimumProfitPercentage"`
}

type TrailingStopControl struct {
	symbol           string
	market           types.Market
	marginSideEffect types.MarginOrderSideEffectType

	trailingStopCallbackRatio fixedpoint.Value
	minimumProfitPercentage   fixedpoint.Value

	CurrentHighestPrice fixedpoint.Value
	OrderID             uint64
}

func (control *TrailingStopControl) IsHigherThanMin(minTargetPrice fixedpoint.Value) bool {
	targetPrice := control.CurrentHighestPrice.Mul(fixedpoint.One.Sub(control.trailingStopCallbackRatio))

	return targetPrice.Compare(minTargetPrice) >= 0
}

func (control *TrailingStopControl) GenerateStopOrder(quantity fixedpoint.Value) types.SubmitOrder {
	targetPrice := control.CurrentHighestPrice.Mul(fixedpoint.One.Sub(control.trailingStopCallbackRatio))

	orderForm := types.SubmitOrder{
		Symbol:           control.symbol,
		Market:           control.market,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeStopLimit,
		Quantity:         quantity,
		MarginSideEffect: control.marginSideEffect,
		TimeInForce:      types.TimeInForceGTC,

		Price:     targetPrice,
		StopPrice: targetPrice,
	}

	return orderForm
}

// Not implemented yet
// ResistanceStop is a kind of stop order by detecting resistance
//type ResistanceStop struct {
//	Interval      types.Interval   `json:"interval"`
//	sensitivity   fixedpoint.Value `json:"sensitivity"`
//	MinVolume     fixedpoint.Value `json:"minVolume"`
//	TakerBuyRatio fixedpoint.Value `json:"takerBuyRatio"`
//}

type Strategy struct {
	*bbgo.Notifiability `json:"-"`
	*bbgo.Persistence
	*bbgo.Graceful `json:"-"`

	Symbol string       `json:"symbol"`
	Market types.Market `json:"-"`

	// Interval for checking support
	Interval types.Interval `json:"interval"`

	// moving average window for checking support (support should be under the moving average line)
	TriggerMovingAverage types.IntervalWindow `json:"triggerMovingAverage"`

	// LongTermMovingAverage is the second moving average line for checking support position
	LongTermMovingAverage types.IntervalWindow `json:"longTermMovingAverage"`

	Quantity              fixedpoint.Value                `json:"quantity"`
	MinVolume             fixedpoint.Value                `json:"minVolume"`
	Sensitivity           fixedpoint.Value                `json:"sensitivity"`
	TakerBuyRatio         fixedpoint.Value                `json:"takerBuyRatio"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
	Targets               []Target                        `json:"targets"`

	// Not implemented yet
	// ResistanceStop *ResistanceStop `json:"resistanceStop"`
	//
	//ResistanceTakerBuyRatio fixedpoint.Value `json:"resistanceTakerBuyRatio"`

	// Min BaseAsset balance to keep
	MinBaseAssetBalance fixedpoint.Value `json:"minBaseAssetBalance"`
	// Max BaseAsset balance to buy
	MaxBaseAssetBalance  fixedpoint.Value `json:"maxBaseAssetBalance"`
	MinQuoteAssetBalance fixedpoint.Value `json:"minQuoteAssetBalance"`

	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity"`

	orderExecutor bbgo.OrderExecutor

	tradeCollector *bbgo.TradeCollector

	orderStore *bbgo.OrderStore
	state      *State

	triggerEMA  *indicator.EWMA
	longTermEMA *indicator.EWMA

	// Trailing stop
	TrailingStopTarget  TrailingStopTarget `json:"trailingStopTarget"`
	trailingStopControl *TrailingStopControl

	// StrategyController
	status types.StrategyStatus
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}

	if s.MinVolume.IsZero() && s.Sensitivity.IsZero() {
		return fmt.Errorf("either minVolume nor sensitivity can not be zero")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.Interval)})

	if s.TriggerMovingAverage != zeroiw {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.TriggerMovingAverage.Interval)})
	}

	if s.LongTermMovingAverage != zeroiw {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.LongTermMovingAverage.Interval)})
	}
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.state.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.state.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.state.Position.Symbol)
	}

	// make it negative
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("order quantity %v is too small, less than %v", quantity, s.Market.MinQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Market:   s.Market,
	}

	s.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)

	createdOrders, err := s.submitOrders(ctx, s.orderExecutor, submitOrder)
	if err != nil {
		log.WithError(err).Errorf("can not place position close order")
	}

	s.orderStore.Add(createdOrders...)
	return err
}

// StrategyController

func (s *Strategy) GetStatus() types.StrategyStatus {
	return s.status
}

func (s *Strategy) Suspend(ctx context.Context) error {
	s.status = types.StrategyStatusStopped

	var err error
	// Cancel all order
	for _, order := range s.orderStore.Orders() {
		err = s.cancelOrder(order.OrderID, ctx, s.orderExecutor)
	}
	if err != nil {
		errMsg := "Not all orders are cancelled! Please check again."
		log.WithError(err).Errorf(errMsg)
		s.Notify(errMsg)
	} else {
		s.Notify("All orders cancelled.")
	}

	// Save state
	if err2 := s.SaveState(); err2 != nil {
		log.WithError(err2).Errorf("can not save state: %+v", s.state)
	} else {
		log.Infof("%s position is saved.", s.Symbol)
	}

	return nil
}

func (s *Strategy) Resume(ctx context.Context) error {
	s.status = types.StrategyStatusRunning

	return nil
}

func (s *Strategy) EmergencyStop(ctx context.Context) error {
	// Close 100% position
	percentage, _ := fixedpoint.NewFromString("100%")
	err := s.ClosePosition(ctx, percentage)

	// Suspend strategy
	_ = s.Suspend(ctx)

	return err
}

func (s *Strategy) SaveState() error {
	if err := s.Persistence.Save(s.state, ID, s.Symbol, stateKey); err != nil {
		return err
	} else {
		log.Infof("state is saved => %+v", s.state)
	}
	return nil
}

func (s *Strategy) LoadState() error {
	var state State

	// load position
	if err := s.Persistence.Load(&state, ID, s.Symbol, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = &State{}
	} else {
		s.state = &state
		log.Infof("state is restored: %+v", s.state)
	}

	if s.state.Position == nil {
		s.state.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.trailingStopControl != nil {
		if s.state.CurrentHighestPrice == nil {
			s.trailingStopControl.CurrentHighestPrice = fixedpoint.NewFromInt(0)
		}
		s.state.CurrentHighestPrice = &s.trailingStopControl.CurrentHighestPrice
	}

	return nil
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, orderForms ...types.SubmitOrder) (types.OrderSlice, error) {
	for _, o := range orderForms {
		s.Notifiability.Notify(o)
	}

	createdOrders, err := orderExecutor.SubmitOrders(ctx, orderForms...)
	if err != nil {
		return nil, err
	}

	s.orderStore.Add(createdOrders...)
	s.tradeCollector.Emit()
	return createdOrders, nil
}

// Cancel order
func (s *Strategy) cancelOrder(orderID uint64, ctx context.Context, orderExecutor bbgo.OrderExecutor) error {
	// Cancel the original order
	order, ok := s.orderStore.Get(orderID)
	if ok {
		switch order.Status {
		case types.OrderStatusCanceled, types.OrderStatusRejected, types.OrderStatusFilled:
			// Do nothing
		default:
			if err := orderExecutor.CancelOrders(ctx, order); err != nil {
				return err
			}
		}
	}

	return nil
}

var slippageModifier = fixedpoint.NewFromFloat(1.003)

func (s *Strategy) calculateQuantity(session *bbgo.ExchangeSession, side types.SideType, closePrice fixedpoint.Value, volume fixedpoint.Value) (fixedpoint.Value, error) {
	var quantity fixedpoint.Value
	if s.Quantity.Sign() > 0 {
		quantity = s.Quantity
	} else if s.ScaleQuantity != nil {
		q, err := s.ScaleQuantity.Scale(closePrice.Float64(), volume.Float64())
		if err != nil {
			return fixedpoint.Zero, err
		}
		quantity = fixedpoint.NewFromFloat(q)
	}

	baseBalance, _ := session.Account.Balance(s.Market.BaseCurrency)
	if side == types.SideTypeSell {
		// quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		if s.MinBaseAssetBalance.Sign() > 0 &&
			baseBalance.Total().Sub(quantity).Compare(s.MinBaseAssetBalance) < 0 {
			quota := baseBalance.Available.Sub(s.MinBaseAssetBalance)
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

	} else if side == types.SideTypeBuy {
		if s.MaxBaseAssetBalance.Sign() > 0 &&
			baseBalance.Total().Add(quantity).Compare(s.MaxBaseAssetBalance) > 0 {
			quota := s.MaxBaseAssetBalance.Sub(baseBalance.Total())
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

		quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
		}

		// for spot, we need to modify the quantity according to the quote balance
		if !session.Margin {
			// add 0.3% for price slippage
			notional := closePrice.Mul(quantity).Mul(slippageModifier)

			if s.MinQuoteAssetBalance.Sign() > 0 &&
				quoteBalance.Available.Sub(notional).Compare(s.MinQuoteAssetBalance) < 0 {
				log.Warnf("modifying quantity %v according to the min quote asset balance %v %s",
					quantity,
					quoteBalance.Available,
					s.Market.QuoteCurrency)
				quota := quoteBalance.Available.Sub(s.MinQuoteAssetBalance)
				quantity = bbgo.AdjustQuantityByMinAmount(quantity, closePrice, quota)
			} else if notional.Compare(quoteBalance.Available) > 0 {
				log.Warnf("modifying quantity %v according to the quote asset balance %v %s",
					quantity,
					quoteBalance.Available,
					s.Market.QuoteCurrency)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quoteBalance.Available)
			}
		}
	}

	return quantity, nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.orderExecutor = orderExecutor

	// StrategyController
	s.status = types.StrategyStatusRunning

	// set default values
	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	if s.Sensitivity.Sign() > 0 {
		volRange, err := s.ScaleQuantity.ByVolumeRule.Range()
		if err != nil {
			return err
		}

		scaleUp := fixedpoint.NewFromFloat(volRange[1])
		scaleLow := fixedpoint.NewFromFloat(volRange[0])
		s.MinVolume = scaleUp.Sub(scaleLow).
			Mul(fixedpoint.One.Sub(s.Sensitivity)).
			Add(scaleLow)
		log.Infof("adjusted minimal support volume to %s according to sensitivity %s", s.MinVolume.String(), s.Sensitivity.String())
	}

	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}
	s.Market = market

	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	if s.TriggerMovingAverage != zeroiw {
		s.triggerEMA = standardIndicatorSet.EWMA(s.TriggerMovingAverage)
	} else {
		s.triggerEMA = standardIndicatorSet.EWMA(types.IntervalWindow{
			Interval: s.Interval,
			Window:   99, // default window
		})
	}

	if s.LongTermMovingAverage != zeroiw {
		s.longTermEMA = standardIndicatorSet.EWMA(s.LongTermMovingAverage)
	}

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	if !s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() {
		s.trailingStopControl = &TrailingStopControl{
			symbol:                    s.Symbol,
			market:                    s.Market,
			marginSideEffect:          s.MarginOrderSideEffect,
			CurrentHighestPrice:       fixedpoint.Zero,
			trailingStopCallbackRatio: s.TrailingStopTarget.TrailingStopCallbackRatio,
			minimumProfitPercentage:   s.TrailingStopTarget.MinimumProfitPercentage,
		}
	}

	if err := s.LoadState(); err != nil {
		return err
	} else {
		s.Notify("%s state is restored => %+v", s.Symbol, s.state)
	}

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)

	if !s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() {
		// Update trailing stop when the position changes
		s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
			// StrategyController
			if s.status != types.StrategyStatusRunning {
				return
			}

			if position.Base.Compare(s.Market.MinQuantity) > 0 { // Update order if we have a position
				// Cancel the original order
				if err := s.cancelOrder(s.trailingStopControl.OrderID, ctx, orderExecutor); err != nil {
					log.WithError(err).Errorf("Can not cancel the original trailing stop order!")
				}
				s.trailingStopControl.OrderID = 0

				// Calculate minimum target price
				var minTargetPrice = fixedpoint.Zero
				if s.trailingStopControl.minimumProfitPercentage.Sign() > 0 {
					minTargetPrice = position.AverageCost.Mul(fixedpoint.One.Add(s.trailingStopControl.minimumProfitPercentage))
				}

				// Place new order if the target price is higher than the minimum target price
				if s.trailingStopControl.IsHigherThanMin(minTargetPrice) {
					orderForm := s.trailingStopControl.GenerateStopOrder(position.Base)
					orders, err := s.submitOrders(ctx, orderExecutor, orderForm)
					if err != nil {
						log.WithError(err).Error("submit profit trailing stop order error")
					} else {
						orderIds := orders.IDs()
						if len(orderIds) > 0 {
							s.trailingStopControl.OrderID = orderIds[0]
						} else {
							log.Error("submit profit trailing stop order error. unknown error")
							s.trailingStopControl.OrderID = 0
						}
					}
				}
			}
			// Save state
			if err := s.SaveState(); err != nil {
				log.WithError(err).Errorf("can not save state: %+v", s.state)
			} else {
				s.Notify("%s position is saved", s.Symbol, s.state.Position)
			}
		})
	}

	s.tradeCollector.BindStream(session.UserDataStream)

	// s.tradeCollector.BindStreamForBackground(session.UserDataStream)
	// go s.tradeCollector.Run(ctx)

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.status != types.StrategyStatusRunning {
			return
		}

		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}
		if kline.Interval != s.Interval {
			return
		}

		closePrice := kline.GetClose()
		highPrice := kline.GetHigh()

		if s.TrailingStopTarget.TrailingStopCallbackRatio.Sign() > 0 {
			if s.state.Position.Base.Compare(s.Market.MinQuantity) <= 0 { // Without a position
				// Update trailing orders with current high price
				s.trailingStopControl.CurrentHighestPrice = highPrice
			} else if s.trailingStopControl.CurrentHighestPrice.Compare(highPrice) < 0 { // With a position
				// Update trailing orders with current high price if it's higher
				s.trailingStopControl.CurrentHighestPrice = highPrice

				// Cancel the original order
				if err := s.cancelOrder(s.trailingStopControl.OrderID, ctx, orderExecutor); err != nil {
					log.WithError(err).Errorf("Can not cancel the original trailing stop order!")
				}
				s.trailingStopControl.OrderID = 0

				// Calculate minimum target price
				var minTargetPrice = fixedpoint.Zero
				if s.trailingStopControl.minimumProfitPercentage.Sign() > 0 {
					minTargetPrice = s.state.Position.AverageCost.Mul(fixedpoint.One.Add(s.trailingStopControl.minimumProfitPercentage))
				}

				// Place new order if the target price is higher than the minimum target price
				if s.trailingStopControl.IsHigherThanMin(minTargetPrice) {
					orderForm := s.trailingStopControl.GenerateStopOrder(s.state.Position.Base)
					orders, err := s.submitOrders(ctx, orderExecutor, orderForm)
					if err != nil {
						log.WithError(err).Error("submit profit trailing stop order error")
					} else {
						s.trailingStopControl.OrderID = orders.IDs()[0]
					}
				}
			}
			// Save state
			if err := s.SaveState(); err != nil {
				log.WithError(err).Errorf("can not save state: %+v", s.state)
			} else {
				s.Notify("%s position is saved", s.Symbol, s.state.Position)
			}
		}

		// check support volume
		if kline.Volume.Compare(s.MinVolume) < 0 {
			return
		}

		// check taker buy ratio, we need strong buy taker
		if s.TakerBuyRatio.Sign() > 0 {
			takerBuyRatio := kline.TakerBuyBaseAssetVolume.Div(kline.Volume)
			takerBuyBaseVolumeThreshold := kline.Volume.Mul(s.TakerBuyRatio)
			if takerBuyRatio.Compare(s.TakerBuyRatio) < 0 {
				s.Notify("%s: taker buy base volume %s (volume ratio %s) is less than %s (volume ratio %s)",
					s.Symbol,
					kline.TakerBuyBaseAssetVolume.String(),
					takerBuyRatio.String(),
					takerBuyBaseVolumeThreshold.String(),
					kline.Volume.String(),
					s.TakerBuyRatio.String(),
					kline,
				)
				return
			}
		}

		if s.longTermEMA != nil && closePrice.Float64() < s.longTermEMA.Last() {
			s.Notify("%s: closed price is below the long term moving average line %f, skipping this support",
				s.Symbol,
				s.longTermEMA.Last(),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && closePrice.Float64() < s.triggerEMA.Last() {
			s.Notify("%s: closed price is above the trigger moving average line %f, skipping this support",
				s.Symbol,
				s.triggerEMA.Last(),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && s.longTermEMA != nil {
			s.Notify("Found %s support: the close price %s is below trigger EMA %f and above long term EMA %f and volume %s > minimum volume %s",
				s.Symbol,
				closePrice.String(),
				s.triggerEMA.Last(),
				s.longTermEMA.Last(),
				kline.Volume.String(),
				s.MinVolume.String(),
				kline)
		} else {
			s.Notify("Found %s support: the close price %s and volume %s > minimum volume %s",
				s.Symbol,
				closePrice.String(),
				kline.Volume.String(),
				s.MinVolume.String(),
				kline)
		}

		quantity, err := s.calculateQuantity(session, types.SideTypeBuy, closePrice, kline.Volume)
		if err != nil {
			log.WithError(err).Errorf("%s quantity calculation error", s.Symbol)
			return
		}

		orderForm := types.SubmitOrder{
			Symbol:           s.Symbol,
			Market:           market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeMarket,
			Quantity:         quantity,
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		s.Notify("Submitting %s market order buy with quantity %s according to the base volume %s, taker buy base volume %s",
			s.Symbol,
			quantity.String(),
			kline.Volume.String(),
			kline.TakerBuyBaseAssetVolume.String(),
			orderForm)

		if _, err := s.submitOrders(ctx, orderExecutor, orderForm); err != nil {
			log.WithError(err).Error("submit order error")
			return
		}
		// Save state
		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s position is saved", s.Symbol, s.state.Position)
		}

		if s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() { // submit fixed target orders
			var targetOrders []types.SubmitOrder
			for _, target := range s.Targets {
				targetPrice := closePrice.Mul(fixedpoint.One.Add(target.ProfitPercentage))
				targetQuantity := quantity.Mul(target.QuantityPercentage)
				targetQuoteQuantity := targetPrice.Mul(targetQuantity)

				if targetQuoteQuantity.Compare(market.MinNotional) <= 0 {
					continue
				}

				if targetQuantity.Compare(market.MinQuantity) <= 0 {
					continue
				}

				targetOrders = append(targetOrders, types.SubmitOrder{
					Symbol:   kline.Symbol,
					Market:   market,
					Type:     types.OrderTypeLimit,
					Side:     types.SideTypeSell,
					Price:    targetPrice,
					Quantity: targetQuantity,

					MarginSideEffect: target.MarginOrderSideEffect,
					TimeInForce:      types.TimeInForceGTC,
				})
			}

			if _, err := s.submitOrders(ctx, orderExecutor, targetOrders...); err != nil {
				log.WithError(err).Error("submit profit target order error")
				return
			}
		}

		s.tradeCollector.Process()
	})

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// Cancel trailing stop order
		if s.TrailingStopTarget.TrailingStopCallbackRatio.Sign() > 0 {
			if err := s.cancelOrder(s.trailingStopControl.OrderID, ctx, orderExecutor); err != nil {
				log.WithError(err).Errorf("Can not cancel the trailing stop order!")
			}
			s.trailingStopControl.OrderID = 0
		}

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s position is saved", s.Symbol, s.state.Position)
		}
	})

	return nil
}
