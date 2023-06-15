package support

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "support"

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
	StopOrder           *types.Order
}

func (control *TrailingStopControl) UpdateCurrentHighestPrice(p fixedpoint.Value) bool {
	orig := control.CurrentHighestPrice
	control.CurrentHighestPrice = fixedpoint.Max(control.CurrentHighestPrice, p)
	return orig.Compare(control.CurrentHighestPrice) == 0
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
// type ResistanceStop struct {
//	Interval      types.Interval   `json:"interval"`
//	sensitivity   fixedpoint.Value `json:"sensitivity"`
//	MinVolume     fixedpoint.Value `json:"minVolume"`
//	TakerBuyRatio fixedpoint.Value `json:"takerBuyRatio"`
// }

type Strategy struct {
	*bbgo.Environment `json:"-"`

	session *bbgo.ExchangeSession

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
	// ResistanceTakerBuyRatio fixedpoint.Value `json:"resistanceTakerBuyRatio"`

	// Min BaseAsset balance to keep
	MinBaseAssetBalance fixedpoint.Value `json:"minBaseAssetBalance"`
	// Max BaseAsset balance to buy
	MaxBaseAssetBalance  fixedpoint.Value `json:"maxBaseAssetBalance"`
	MinQuoteAssetBalance fixedpoint.Value `json:"minQuoteAssetBalance"`

	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity"`

	orderExecutor *bbgo.GeneralOrderExecutor

	Position            *types.Position    `persistence:"position"`
	ProfitStats         *types.ProfitStats `persistence:"profit_stats"`
	TradeStats          *types.TradeStats  `persistence:"trade_stats"`
	CurrentHighestPrice fixedpoint.Value   `persistence:"current_highest_price"`

	triggerEMA  *indicator.EWMA
	longTermEMA *indicator.EWMA

	// Trailing stop
	TrailingStopTarget  TrailingStopTarget `json:"trailingStopTarget"`
	trailingStopControl *TrailingStopControl

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
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
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})

	if s.TriggerMovingAverage != zeroiw {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.TriggerMovingAverage.Interval})
	}

	if s.LongTermMovingAverage != zeroiw {
		session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LongTermMovingAverage.Interval})
	}
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
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

	bbgo.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)
	_, err := s.orderExecutor.SubmitOrders(ctx, submitOrder)
	return err
}

func (s *Strategy) submitOrders(ctx context.Context, orderExecutor bbgo.OrderExecutor, orderForms ...types.SubmitOrder) (types.OrderSlice, error) {
	return s.orderExecutor.SubmitOrders(ctx, orderForms...)
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

	baseBalance, _ := session.GetAccount().Balance(s.Market.BaseCurrency)
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

		quoteBalance, ok := session.GetAccount().Balance(s.Market.QuoteCurrency)
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
	s.session = session
	instanceID := s.InstanceID()

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	// trade stats
	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.Bind()

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		// Cancel all order
		_ = s.orderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})

	s.OnEmergencyStop(func() {
		// Close 100% position
		percentage := fixedpoint.NewFromFloat(1.0)
		if err := s.ClosePosition(context.Background(), percentage); err != nil {
			errMsg := "failed to close position"
			log.WithError(err).Errorf(errMsg)
			bbgo.Notify(errMsg)
		}

		if err := s.Suspend(); err != nil {
			errMsg := "failed to suspend strategy"
			log.WithError(err).Errorf(errMsg)
			bbgo.Notify(errMsg)
		}
	})

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

	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)

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

	if !s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() {
		s.trailingStopControl = &TrailingStopControl{
			symbol:                    s.Symbol,
			market:                    s.Market,
			marginSideEffect:          s.MarginOrderSideEffect,
			trailingStopCallbackRatio: s.TrailingStopTarget.TrailingStopCallbackRatio,
			minimumProfitPercentage:   s.TrailingStopTarget.MinimumProfitPercentage,
			CurrentHighestPrice:       s.CurrentHighestPrice,
		}
	}

	if !s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() {
		// Update trailing stop when the position changes
		s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
			// StrategyController
			if s.Status != types.StrategyStatusRunning {
				return
			}

			if !position.IsLong() || position.IsDust(position.AverageCost) {
				return
			}

			s.updateStopOrder(ctx)
		})
	}

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// StrategyController
		if s.Status != types.StrategyStatusRunning {
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

		// check our trailing stop
		if s.TrailingStopTarget.TrailingStopCallbackRatio.Sign() > 0 {
			if s.Position.IsLong() && !s.Position.IsDust(closePrice) {
				changed := s.trailingStopControl.UpdateCurrentHighestPrice(highPrice)
				if changed {
					// Cancel the original order
					s.updateStopOrder(ctx)
				}
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
				bbgo.Notify("%s: taker buy base volume %s (volume ratio %s) is less than %s (volume ratio %s)",
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

		if s.longTermEMA != nil && closePrice.Float64() < s.longTermEMA.Last(0) {
			bbgo.Notify("%s: closed price is below the long term moving average line %f, skipping this support",
				s.Symbol,
				s.longTermEMA.Last(0),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && closePrice.Float64() > s.triggerEMA.Last(0) {
			bbgo.Notify("%s: closed price is above the trigger moving average line %f, skipping this support",
				s.Symbol,
				s.triggerEMA.Last(0),
				kline,
			)
			return
		}

		if s.triggerEMA != nil && s.longTermEMA != nil {
			bbgo.Notify("Found %s support: the close price %s is below trigger EMA %f and above long term EMA %f and volume %s > minimum volume %s",
				s.Symbol,
				closePrice.String(),
				s.triggerEMA.Last(0),
				s.longTermEMA.Last(0),
				kline.Volume.String(),
				s.MinVolume.String(),
				kline)
		} else {
			bbgo.Notify("Found %s support: the close price %s and volume %s > minimum volume %s",
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
			Market:           s.Market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeMarket,
			Quantity:         quantity,
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		bbgo.Notify("Submitting %s market order buy with quantity %s according to the base volume %s, taker buy base volume %s",
			s.Symbol,
			quantity.String(),
			kline.Volume.String(),
			kline.TakerBuyBaseAssetVolume.String(),
			orderForm)

		if _, err := s.submitOrders(ctx, orderExecutor, orderForm); err != nil {
			log.WithError(err).Error("submit order error")
			return
		}

		if s.TrailingStopTarget.TrailingStopCallbackRatio.IsZero() { // submit fixed target orders
			var targetOrders []types.SubmitOrder
			for _, target := range s.Targets {
				targetPrice := closePrice.Mul(fixedpoint.One.Add(target.ProfitPercentage))
				targetQuantity := quantity.Mul(target.QuantityPercentage)
				targetQuoteQuantity := targetPrice.Mul(targetQuantity)

				if targetQuoteQuantity.Compare(s.Market.MinNotional) <= 0 {
					continue
				}

				if targetQuantity.Compare(s.Market.MinQuantity) <= 0 {
					continue
				}

				targetOrders = append(targetOrders, types.SubmitOrder{
					Symbol:   kline.Symbol,
					Market:   s.Market,
					Type:     types.OrderTypeLimit,
					Side:     types.SideTypeSell,
					Price:    targetPrice,
					Quantity: targetQuantity,

					MarginSideEffect: target.MarginOrderSideEffect,
					TimeInForce:      types.TimeInForceGTC,
				})
			}

			_, err = s.orderExecutor.SubmitOrders(ctx, targetOrders...)
			if err != nil {
				bbgo.Notify("submit %s profit trailing stop order error: %s", s.Symbol, err.Error())
			}
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		// Cancel trailing stop order
		if s.TrailingStopTarget.TrailingStopCallbackRatio.Sign() > 0 {
			_ = s.orderExecutor.GracefulCancel(ctx)
		}
	})

	return nil
}

func (s *Strategy) updateStopOrder(ctx context.Context) {
	// cancel the original stop order
	if s.trailingStopControl.StopOrder != nil {
		if err := s.session.Exchange.CancelOrders(ctx, *s.trailingStopControl.StopOrder); err != nil {
			log.WithError(err).Error("cancel order error")
		}
		s.trailingStopControl.StopOrder = nil
		s.orderExecutor.TradeCollector().Process()
	}

	// Calculate minimum target price
	var minTargetPrice = fixedpoint.Zero
	if s.trailingStopControl.minimumProfitPercentage.Sign() > 0 {
		minTargetPrice = s.Position.AverageCost.Mul(fixedpoint.One.Add(s.trailingStopControl.minimumProfitPercentage))
	}

	// Place new order if the target price is higher than the minimum target price
	if s.trailingStopControl.IsHigherThanMin(minTargetPrice) {
		orderForm := s.trailingStopControl.GenerateStopOrder(s.Position.Base)
		orders, err := s.orderExecutor.SubmitOrders(ctx, orderForm)
		if err != nil {
			bbgo.Notify("failed to submit the trailing stop order on %s", s.Symbol)
			log.WithError(err).Error("submit profit trailing stop order error")
		}

		if len(orders) == 0 {
			log.Error("unexpected error: len(createdOrders) = 0")
			return
		}

		s.trailingStopControl.StopOrder = &orders[0]
	}
}
