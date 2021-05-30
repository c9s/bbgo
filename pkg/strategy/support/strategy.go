package support

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "support"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Target struct {
	ProfitPercentage      float64                         `json:"profitPercentage"`
	QuantityPercentage    float64                         `json:"quantityPercentage"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
}

type Strategy struct {
	*bbgo.Notifiability

	Symbol                string                          `json:"symbol"`
	Interval              types.Interval                  `json:"interval"`
	MovingAverageWindow   int                             `json:"movingAverageWindow"`
	Quantity              fixedpoint.Value                `json:"quantity"`
	MinVolume             fixedpoint.Value                `json:"minVolume"`
	TakerBuyRatio         fixedpoint.Value                `json:"takerBuyRatio"`
	MarginOrderSideEffect types.MarginOrderSideEffectType `json:"marginOrderSideEffect"`
	Targets               []Target                        `json:"targets"`

	// Max BaseAsset balance to buy
	MaxBaseAssetBalance  fixedpoint.Value `json:"maxBaseAssetBalance"`
	MinQuoteAssetBalance fixedpoint.Value `json:"minQuoteAssetBalance"`

	ScaleQuantity *bbgo.PriceVolumeScale `json:"scaleQuantity"`

	orderStore *bbgo.OrderStore
	tradeStore *bbgo.TradeStore
	tradeC     chan types.Trade
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.Quantity == 0 && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}

	if s.MinVolume == 0 {
		return fmt.Errorf("minVolume can not be zero")
	}

	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: string(s.Interval)})
}

func (s *Strategy) handleTradeUpdate(trade types.Trade) {
	s.tradeC <- trade
}

func (s *Strategy) tradeCollector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case trade := <-s.tradeC:
			s.tradeStore.Add(trade)

		}
	}
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// buffer 100 trades in the channel
	s.tradeC = make(chan types.Trade, 100)
	s.tradeStore = bbgo.NewTradeStore(s.Symbol)

	// set default values
	if s.Interval == "" {
		s.Interval = types.Interval5m
	}

	if s.MovingAverageWindow == 0 {
		s.MovingAverageWindow = 99
	}

	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}

	standardIndicatorSet, ok := session.StandardIndicatorSet(s.Symbol)
	if !ok {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	s.orderStore = bbgo.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(session.UserDataStream)

	var iw = types.IntervalWindow{Interval: s.Interval, Window: s.MovingAverageWindow}
	var ema = standardIndicatorSet.EWMA(iw)

	go s.tradeCollector(ctx)

	session.UserDataStream.OnTradeUpdate(s.handleTradeUpdate)
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		closePriceF := kline.GetClose()
		if closePriceF > ema.Last() {
			return
		}

		closePrice := fixedpoint.NewFromFloat(closePriceF)

		if kline.Volume < s.MinVolume.Float64() {
			return
		}

		if s.TakerBuyRatio > 0 {
			takerBuyBaseVolumeThreshold := kline.Volume * s.TakerBuyRatio.Float64()
			if kline.TakerBuyBaseAssetVolume < takerBuyBaseVolumeThreshold {
				s.Notify("%s: taker buy base volume %f is less than %f (volume ratio %f)",
					s.Symbol,
					kline.TakerBuyBaseAssetVolume,
					takerBuyBaseVolumeThreshold,
					s.TakerBuyRatio.Float64(),
				)
			}
		}

		s.Notify("Found %s support: the close price %f is under EMA %f and volume %f > minimum volume %f",
			s.Symbol,
			closePrice.Float64(),
			ema.Last(),
			kline.Volume,
			s.MinVolume.Float64(), kline)

		var quantity fixedpoint.Value
		if s.Quantity > 0 {
			quantity = s.Quantity
		} else if s.ScaleQuantity != nil {
			qf, err := s.ScaleQuantity.Scale(closePrice.Float64(), kline.Volume)
			if err != nil {
				log.WithError(err).Error(err.Error())
				return
			}
			quantity = fixedpoint.NewFromFloat(qf)
		}

		baseBalance, _ := session.Account.Balance(market.BaseCurrency)
		if s.MaxBaseAssetBalance > 0 && baseBalance.Total()+quantity > s.MaxBaseAssetBalance {
			quota := s.MaxBaseAssetBalance - baseBalance.Total()
			quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quota)
		}

		quoteBalance, ok := session.Account.Balance(market.QuoteCurrency)
		if !ok {
			log.Errorf("quote balance %s not found", market.QuoteCurrency)
			return
		}

		// for spot, we need to modify the quantity according to the quote balance
		if !session.Margin {
			// add 0.3% for price slippage
			notional := closePrice.Mul(quantity).MulFloat64(1.003)

			if s.MinQuoteAssetBalance > 0 && quoteBalance.Available-notional < s.MinQuoteAssetBalance {
				log.Warnf("modifying quantity %f according to the min quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					market.QuoteCurrency)
				quota := quoteBalance.Available - s.MinQuoteAssetBalance
				quantity = bbgo.AdjustQuantityByMinAmount(quantity, closePrice, quota)
			} else if notional > quoteBalance.Available {
				log.Warnf("modifying quantity %f according to the quote asset balance %f %s",
					quantity.Float64(),
					quoteBalance.Available.Float64(),
					market.QuoteCurrency)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, closePrice, quoteBalance.Available)
			}
		}

		s.Notify("Submitting %s market order buy with quantity %f according to the base volume %f, taker buy base volume %f",
			s.Symbol,
			quantity,
			kline.Volume,
			kline.TakerBuyBaseAssetVolume)

		orderForm := types.SubmitOrder{
			Symbol:           s.Symbol,
			Market:           market,
			Side:             types.SideTypeBuy,
			Type:             types.OrderTypeMarket,
			Quantity:         quantity.Float64(),
			MarginSideEffect: s.MarginOrderSideEffect,
		}

		createdOrders, err := orderExecutor.SubmitOrders(ctx, orderForm)
		if err != nil {
			log.WithError(err).Error("submit order error")
			return
		}
		s.orderStore.Add(createdOrders...)

		// submit target orders
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

		_, err = orderExecutor.SubmitOrders(ctx, targetOrders...)
		if err != nil {
			log.WithError(err).Error("submit profit target order error")
		}
	})

	return nil
}
