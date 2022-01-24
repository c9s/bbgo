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
	Position *types.Position `json:"position,omitempty"`
}

type Target struct {
	ProfitPercentage      float64                         `json:"profitPercentage"`
	QuantityPercentage    float64                         `json:"quantityPercentage"`
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
		targetPrice := price.Float64() * (1.0 + target.ProfitPercentage)
		targetQuantity := quantity.Float64() * target.QuantityPercentage
		targetQuoteQuantity := targetPrice * targetQuantity

		if targetQuoteQuantity <= market.MinNotional {
			continue
		}

		if targetQuantity <= market.MinQuantity {
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
			TimeInForce:      "GTC",
		})
	}

	return targetOrders
}

type TrailingStopTarget struct {
	TrailingStopCallBackRatio float64 `json:"trailingStopCallBackRatio"`
	MinimumProfitPercentage   float64 `json:"minimumProfitPercentage"`
}

type TrailingStopControl struct {
	symbol           string
	market           types.Market
	marginSideEffect types.MarginOrderSideEffectType

	trailingStopCallBackRatio float64
	minimumProfitPercentage   float64

	CurrentHighestPrice fixedpoint.Value
	OrderID             uint64
}

func NewTrailingStopControl(symbol string, market types.Market, marginSideEffect types.MarginOrderSideEffectType, trailingStopCallBackRatio float64, minimumProfitPercentage float64) *TrailingStopControl {
	return &TrailingStopControl{
		symbol:                    symbol,
		market:                    market,
		marginSideEffect:          marginSideEffect,
		CurrentHighestPrice:       fixedpoint.NewFromInt(0),
		trailingStopCallBackRatio: trailingStopCallBackRatio,
		minimumProfitPercentage:   minimumProfitPercentage,
	}
}

func (control *TrailingStopControl) IsHigherThanMin(minTargetPrice float64) bool {
	targetPrice := control.CurrentHighestPrice.Float64() * (1 - control.trailingStopCallBackRatio)

	return targetPrice >= minTargetPrice
}

func (control *TrailingStopControl) GenerateStopOrder(quantity float64) types.SubmitOrder {
	targetPrice := control.CurrentHighestPrice.Float64() * (1 - control.trailingStopCallBackRatio)

	orderForm := types.SubmitOrder{
		Symbol:           control.symbol,
		Market:           control.market,
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeStopLimit,
		Quantity:         quantity,
		MarginSideEffect: control.marginSideEffect,
		TimeInForce:      "GTC",

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

	tradeCollector *bbgo.TradeCollector

	orderStore *bbgo.OrderStore
	state      *State

	triggerEMA  *indicator.EWMA
	longTermEMA *indicator.EWMA

	// Trailing stop
	TrailingStopTarget  TrailingStopTarget `json:"trailingStopTarget"`
	trailingStopControl *TrailingStopControl
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Quantity == 0 && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}

	if s.MinVolume == 0 && s.Sensitivity == 0 {
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
func (s *Strategy) cancelOrder(orderID uint64, ctx context.Context, session *bbgo.ExchangeSession) error {
	// Cancel the original order
	order, ok := s.orderStore.Get(orderID)
	if ok {
		switch order.Status {
		case types.OrderStatusCanceled, types.OrderStatusRejected, types.OrderStatusFilled:
			// Do nothing
		default:
			if err := session.Exchange.CancelOrders(ctx, order); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Strategy) calculateQuantity(session *bbgo.ExchangeSession, side types.SideType, closePrice fixedpoint.Value, volume float64) (fixedpoint.Value, error) {
	var quantity fixedpoint.Value
	if s.Quantity > 0 {
		quantity = s.Quantity
	} else if s.ScaleQuantity != nil {
		qf, err := s.ScaleQuantity.Scale(closePrice.Float64(), volume)
		if err != nil {
			return 0, err
		}
		quantity = fixedpoint.NewFromFloat(qf)
	}

	baseBalance, _ := session.Account.Balance(s.Market.BaseCurrency)
	if side == types.SideTypeSell {
		// quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		if s.MinBaseAssetBalance > 0 && (baseBalance.Total()-quantity) < s.MinBaseAssetBalance {
			quota := baseBalance.Available - s.MinBaseAssetBalance
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

	} else if side == types.SideTypeBuy {
		if s.MaxBaseAssetBalance > 0 && baseBalance.Total()+quantity > s.MaxBaseAssetBalance {
			quota := s.MaxBaseAssetBalance - baseBalance.Total()
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

		quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
		if !ok {
			return 0, fmt.Errorf("quote balance %s not found", s.Market.QuoteCurrency)
		}

		// for spot, we need to modify the quantity according to the quote balance
		if !session.Margin {
			// add 0.3% for price slippage
			notional := closePrice.Mul(quantity).MulFloat64(1.003)

			if s.MinQuoteAssetBalance > 0 && quoteBalance.Available-notional < s.MinQuoteAssetBalance {
				log.Warnf("modifying quantity %f according to the min quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					s.Market.QuoteCurrency)
				quota := quoteBalance.Available - s.MinQuoteAssetBalance
				quantity = bbgo.AdjustQuantityByMinAmount(quantity, closePrice, quota)
			} else if notional > quoteBalance.Available {
				log.Warnf("modifying quantity %f according to the quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					s.Market.QuoteCurrency)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quoteBalance.Available)
			}
		}
	}

	return quantity, nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// set default values
	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	if s.Sensitivity > 0 {
		volRange, err := s.ScaleQuantity.ByVolumeRule.Range()
		if err != nil {
			return err
		}

		s.MinVolume = fixedpoint.NewFromFloat(volRange[0]) + fixedpoint.NewFromFloat(volRange[1]-volRange[0]).Mul(fixedpoint.NewFromFloat(1.0)-s.Sensitivity)
		log.Infof("adjusted minimal support volume to %f according to sensitivity %f", s.MinVolume.Float64(), s.Sensitivity.Float64())
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

	if err := s.LoadState(); err != nil {
		return err
	} else {
		s.Notify("%s state is restored => %+v", s.Symbol, s.state)
	}

	if s.TrailingStopTarget.TrailingStopCallBackRatio != 0 {
		s.trailingStopControl = NewTrailingStopControl(s.Symbol, s.Market, s.MarginOrderSideEffect, s.TrailingStopTarget.TrailingStopCallBackRatio, s.TrailingStopTarget.MinimumProfitPercentage)
	}

	s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.state.Position, s.orderStore)

	if s.TrailingStopTarget.TrailingStopCallBackRatio != 0 {
		// Update trailing stop when the position changes
		s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
			if position.Base.Float64() > 0 { // Update order if we have a position
				// Cancel the original order
				if err := s.cancelOrder(s.trailingStopControl.OrderID, ctx, session); err != nil {
					log.WithError(err).Errorf("Can not cancel the original trailing stop order!")
				}
				s.trailingStopControl.OrderID = 0

				// Calculate minimum target price
				var minTargetPrice float64 = 0.0
				if s.trailingStopControl.minimumProfitPercentage > 0 {
					minTargetPrice = position.AverageCost.Float64() * (1 + s.trailingStopControl.minimumProfitPercentage)
				}

				// Place new order if the target price is higher than the minimum target price
				if s.trailingStopControl.IsHigherThanMin(minTargetPrice) {
					orderForm := s.trailingStopControl.GenerateStopOrder(position.Base.Float64())
					orders, err := s.submitOrders(ctx, orderExecutor, orderForm)
					if err != nil {
						log.WithError(err).Error("submit profit trailing stop order error")
					} else {
						s.trailingStopControl.OrderID = orders.IDs()[0]
					}
				}
			}
		})
	}

	s.tradeCollector.BindStream(session.UserDataStream)

	// s.tradeCollector.BindStreamForBackground(session.UserDataStream)
	// go s.tradeCollector.Run(ctx)

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}
		if kline.Interval != s.Interval {
			return
		}

		closePriceF := kline.GetClose()
		closePrice := fixedpoint.NewFromFloat(closePriceF)
		highPriceF := kline.GetHigh()
		highPrice := fixedpoint.NewFromFloat(highPriceF)

		if s.TrailingStopTarget.TrailingStopCallBackRatio > 0 {
			if s.state.Position.Base.Float64() <= 0 { // Without a position
				// Update trailing orders with current high price
				s.trailingStopControl.CurrentHighestPrice = highPrice
			} else if s.trailingStopControl.CurrentHighestPrice.Float64() < highPriceF { // With a position
				// Update trailing orders with current high price if it's higher
				s.trailingStopControl.CurrentHighestPrice = highPrice

				// Cancel the original order
				if err := s.cancelOrder(s.trailingStopControl.OrderID, ctx, session); err != nil {
					log.WithError(err).Errorf("Can not cancel the original trailing stop order!")
				}
				s.trailingStopControl.OrderID = 0

				// Calculate minimum target price
				var minTargetPrice float64 = 0.0
				if s.trailingStopControl.minimumProfitPercentage > 0 {
					minTargetPrice = s.state.Position.AverageCost.Float64() * (1 + s.trailingStopControl.minimumProfitPercentage)
				}

				// Place new order if the target price is higher than the minimum target price
				if s.trailingStopControl.IsHigherThanMin(minTargetPrice) {
					orderForm := s.trailingStopControl.GenerateStopOrder(s.state.Position.Base.Float64())
					orders, err := s.submitOrders(ctx, orderExecutor, orderForm)
					if err != nil {
						log.WithError(err).Error("submit profit trailing stop order error")
					} else {
						s.trailingStopControl.OrderID = orders.IDs()[0]
					}
				}
			}
		}

		// check support volume
		if kline.Volume < s.MinVolume.Float64() {
			return
		}

		// check taker buy ratio, we need strong buy taker
		if s.TakerBuyRatio > 0 {
			takerBuyRatio := kline.TakerBuyBaseAssetVolume / kline.Volume
			takerBuyBaseVolumeThreshold := kline.Volume * s.TakerBuyRatio.Float64()
			if takerBuyRatio < s.TakerBuyRatio.Float64() {
				s.Notify("%s: taker buy base volume %f (volume ratio %f) is less than %f (volume ratio %f)",
					s.Symbol,
					kline.TakerBuyBaseAssetVolume,
					takerBuyRatio,
					takerBuyBaseVolumeThreshold,
					kline.Volume,
					s.TakerBuyRatio.Float64(),
					kline,
				)
				return
			}
		}

		if s.longTermEMA != nil && closePriceF < s.longTermEMA.Last() {
			s.Notify("%s: closed price is below the long term moving average line %f, skipping this support",
				s.Symbol,
				s.longTermEMA.Last(),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && closePriceF > s.triggerEMA.Last() {
			s.Notify("%s: closed price is above the trigger moving average line %f, skipping this support",
				s.Symbol,
				s.triggerEMA.Last(),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && s.longTermEMA != nil {
			s.Notify("Found %s support: the close price %f is below trigger EMA %f and above long term EMA %f and volume %f > minimum volume %f",
				s.Symbol,
				closePrice.Float64(),
				s.triggerEMA.Last(),
				s.longTermEMA.Last(),
				kline.Volume,
				s.MinVolume.Float64(),
				kline)
		} else {
			s.Notify("Found %s support: the close price %f and volume %f > minimum volume %f",
				s.Symbol,
				closePrice.Float64(),
				kline.Volume,
				s.MinVolume.Float64(),
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
			Quantity:         quantity.Float64(),
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		s.Notify("Submitting %s market order buy with quantity %f according to the base volume %f, taker buy base volume %f",
			s.Symbol,
			quantity.Float64(),
			kline.Volume,
			kline.TakerBuyBaseAssetVolume,
			orderForm)

		if _, err := s.submitOrders(ctx, orderExecutor, orderForm); err != nil {
			log.WithError(err).Error("submit order error")
			return
		}

		if s.TrailingStopTarget.TrailingStopCallBackRatio == 0 { // submit fixed target orders
			var targetOrders []types.SubmitOrder
			for _, target := range s.Targets {
				targetPrice := closePrice.Float64() * (1.0 + target.ProfitPercentage)
				targetQuantity := quantity.Float64() * target.QuantityPercentage
				targetQuoteQuantity := targetPrice * targetQuantity

				if targetQuoteQuantity <= market.MinNotional {
					continue
				}

				if targetQuantity <= market.MinQuantity {
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
					TimeInForce:      "GTC",
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

		if err := s.SaveState(); err != nil {
			log.WithError(err).Errorf("can not save state: %+v", s.state)
		} else {
			s.Notify("%s position is saved", s.Symbol, s.state.Position)
		}
	})

	return nil
}
