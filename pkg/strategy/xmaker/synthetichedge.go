package xmaker

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
)

const TradeTagMock = "mock"

// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
// SourceSymbol could be something like binance.BTCUSDT
// FiatSymbol could be something like max.USDTTWD
type SyntheticHedge struct {
	// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
	Enabled bool `json:"enabled"`

	Source *HedgeMarketConfig `json:"source"`
	Fiat   *HedgeMarketConfig `json:"fiat"`

	sourceMarket, fiatMarket *HedgeMarket

	syntheticTradeId uint64

	logger logrus.FieldLogger

	mu sync.Mutex

	environment *bbgo.Environment

	strategy *Strategy

	forward bool
}

// InitializeAndBind
// not a good way to initialize the synthetic hedge with the strategy instance
// but we need to build the trade collector to update the profit and position
func (s *SyntheticHedge) InitializeAndBind(sessions map[string]*bbgo.ExchangeSession, strategy *Strategy) error {
	if !s.Enabled {
		return nil
	}

	// Initialize the synthetic quote
	if s.Source == nil || s.Fiat == nil {
		return fmt.Errorf("source and fiat must be set")
	}

	if strategy != nil && strategy.logger != nil {
		s.logger = strategy.logger.WithFields(logrus.Fields{
			"feature": "synthetic_hedge",
		})
	} else {
		s.logger = log.WithFields(logrus.Fields{
			"feature": "synthetic_hedge",
		})
	}

	var err error

	s.sourceMarket, err = initializeHedgeMarketFromConfig(s.Source, sessions)
	if err != nil {
		return err
	}

	s.sourceMarket.SetLogger(s.logger)

	s.fiatMarket, err = initializeHedgeMarketFromConfig(s.Fiat, sessions)
	if err != nil {
		return err
	}

	s.fiatMarket.SetLogger(s.logger)

	// update strategy instance ID for the source and fiat markets
	s.sourceMarket.Position.StrategyInstanceID = strategy.InstanceID()
	s.fiatMarket.Position.StrategyInstanceID = strategy.InstanceID()
	s.strategy = strategy

	// BTC/USDT -> USDT/TWD = forward
	// BTC/USDT -> USDC/USDT = reverse (!forward)
	s.forward = s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency

	s.sourceMarket.positionExposure.OnCover(strategy.positionExposure.Cover)
	s.sourceMarket.positionExposure.OnClose(strategy.positionExposure.Close)
	return s.initialize(strategy)
}

func (s *SyntheticHedge) initialize(strategy *Strategy) error {
	s.environment = strategy.Environment

	// when receiving trades from the source session,
	// mock a trade with the quote amount and add to the fiat position
	//
	// hedge flow:
	//   maker trade -> update source market position delta
	//   source market position delta -> convert to fiat trade -> update fiat market position delta
	s.sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		cost := fixedpoint.Zero

		// 1) get the fiat book price from the snapshot when possible
		bid, ask := s.fiatMarket.getQuotePrice()

		// 2) build up fiat position from the trade quote quantity
		fiatQuantity := trade.QuoteQuantity
		fiatDelta := fiatQuantity

		// reverse side because we assume source's quote currency is the fiat market's base currency here
		fiatOrderSide := trade.Side

		if s.forward {
			fiatQuantity = trade.QuoteQuantity
			fiatOrderSide = trade.Side
		} else {
			fiatQuantity = fixedpoint.One.Div(trade.QuoteQuantity)
			fiatOrderSide = trade.Side.Reverse()
		}

		fiatBaseSide := fiatOrderSide.Reverse()

		switch fiatOrderSide {
		case types.SideTypeBuy:
			cost = ask
			fiatDelta = fiatQuantity.Neg()
		case types.SideTypeSell:
			cost = bid
			fiatDelta = fiatQuantity
		default:
			return
		}

		// 3) convert source trade into fiat trade as the average cost of the fiat market position
		// This assumes source's quote currency is the fiat market's base currency
		fiatTrade := s.fiatMarket.newMockTrade(fiatBaseSide, cost, fiatQuantity, trade.Time.Time())
		if profit, netProfit, madeProfit := s.fiatMarket.Position.AddTrade(fiatTrade); madeProfit {
			// TODO: record the profits in somewhere?
			_ = profit
			_ = netProfit
		}

		// add position delta to let the fiat market hedge the position
		s.fiatMarket.positionDeltaC <- fiatDelta
	})

	s.fiatMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		avgCost := s.sourceMarket.Position.AverageCost

		// convert the trade quantity to the source market's base currency quantity
		// calculate how much base quantity we can close the source position
		baseQuantity := fixedpoint.Zero
		if s.forward {
			baseQuantity = trade.Quantity.Div(avgCost)
		} else {
			// To handle the case where the source market's quote currency is not the fiat market's base currency:
			// Assume it's TWD/USDT, the exchange rate is 0.03125 when USDT/TWD is at 32.0,
			// and trade quantity is in TWD,
			// so we need to convert Quantity from TWD to USDT unit by div
			baseQuantity = avgCost.Div(trade.Quantity.Div(trade.Price))
		}

		// convert the fiat trade into source trade to close the source position
		// side is reversed because we are closing the source hedge position
		mockSourceTrade := s.sourceMarket.newMockTrade(trade.Side.Reverse(), avgCost, baseQuantity, trade.Time.Time())
		if profit, netProfit, madeProfit := s.sourceMarket.Position.AddTrade(mockSourceTrade); madeProfit {
			_ = profit
			_ = netProfit
		}

		// close the maker position
		// create a mock trade for closing the maker position
		// This assumes the source market's quote currency is the fiat market's base currency
		price := fixedpoint.Zero
		if s.forward {
			price = avgCost.Mul(trade.Price)
		} else {
			price = avgCost.Div(trade.Price)
		}

		side := trade.Side
		tradeId := tradeid.GlobalGenerator.Generate()

		syntheticTrade := types.Trade{
			ID:            tradeId,
			OrderID:       tradeId,
			Exchange:      strategy.makerSession.ExchangeName,
			Symbol:        strategy.makerMarket.Symbol,
			Price:         price,
			Quantity:      baseQuantity,
			QuoteQuantity: baseQuantity.Mul(price),
			Side:          side,
			IsBuyer:       side == types.SideTypeBuy,
			IsMaker:       false,
			Time:          trade.Time,
			Fee:           trade.Fee,
			FeeCurrency:   trade.FeeCurrency,
			Tag:           TradeTagMock,
		}
		s.logger.Infof("[syntheticHedge] synthetic trade created: %+v, average cost: %f", syntheticTrade, avgCost.Float64())
		syntheticOrder := newMockOrderFromTrade(syntheticTrade, types.OrderTypeMarket)
		strategy.orderStore.Add(syntheticOrder)
		strategy.tradeCollector.TradeStore().Add(syntheticTrade)
		strategy.tradeCollector.Process()
	})

	return nil
}

// Hedge is the main function to perform the synthetic hedging:
// 1) use the snapshot price as the source average cost
// 2) submit the hedge order to the source exchange
// 3) query trades from of the hedge order.
// 4) build up the source hedge position for the average cost.
// 5) submit fiat hedge order to the fiat market to convert the quote.
// 6) merge the positions.
func (s *SyntheticHedge) Hedge(
	_ context.Context,
	uncoveredPosition fixedpoint.Value,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	s.logger.Infof("[syntheticHedge] synthetic hedging with delta: %f", uncoveredPosition.Float64())
	s.sourceMarket.positionDeltaC <- uncoveredPosition
	return nil
}

func (s *SyntheticHedge) GetQuotePrices() (fixedpoint.Value, fixedpoint.Value, bool) {
	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	bid, ask := s.sourceMarket.getQuotePrice()
	bid2, ask2 := s.fiatMarket.getQuotePrice()

	if s.forward {
		sBid := bid.Mul(bid2)
		sAsk := ask.Mul(ask2)

		log.Infof("synthetic quote:[%f/%f] source:[%f/%f] fiat:[%f/%f]",
			sAsk.Float64(), sBid.Float64(),
			ask.Float64(), bid.Float64(),
			ask2.Float64(), bid2.Float64())
		return sBid, sAsk, sBid.Sign() > 0 && sAsk.Sign() > 0
	} else {
		sBid := bid.Div(bid2)
		sAsk := ask.Div(ask2)

		log.Infof("synthetic quote:[%f/%f] source:[%f/%f] fiat:[%f/%f]",
			sAsk.Float64(), sBid.Float64(),
			ask.Float64(), bid.Float64(),
			ask2.Float64(), bid2.Float64())
		return sBid, sAsk, sBid.Sign() > 0 && sAsk.Sign() > 0
	}
}

func (s *SyntheticHedge) Stop(shutdownCtx context.Context) error {
	s.logger.Infof("[syntheticHedge] stopping synthetic hedge workers")
	s.sourceMarket.Stop(shutdownCtx)
	s.fiatMarket.Stop(shutdownCtx)

	instanceID := ID + "-synthetichedge"
	if s.strategy != nil {
		instanceID = s.strategy.InstanceID()
	}

	s.sourceMarket.Sync(s.sourceMarket.tradingCtx, instanceID)
	s.fiatMarket.Sync(s.fiatMarket.tradingCtx, instanceID)

	s.logger.Infof("[syntheticHedge] synthetic hedge workers stopped")
	return nil
}

func (s *SyntheticHedge) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fmt.Errorf("sourceMarket and fiatMarket must be initialized")
	}

	instanceID := ID
	if s.strategy != nil {
		instanceID = s.strategy.InstanceID()
	}

	if err := s.sourceMarket.Restore(ctx, instanceID); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to restore source market persistence")
		return err
	}

	if err := s.fiatMarket.Restore(ctx, instanceID); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to restore fiat market persistence")
		return err
	}

	if err := s.sourceMarket.Start(ctx); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to start")
		return err
	}

	if err := s.fiatMarket.Start(ctx); err != nil {
		s.logger.WithError(err).Errorf("[syntheticHedge] failed to start")
		return err
	}

	s.sourceMarket.WaitForReady(ctx)
	s.fiatMarket.WaitForReady(ctx)

	s.logger.Infof("[syntheticHedge] source market and fiat market are ready")
	return nil
}

func newMockOrderFromTrade(trade types.Trade, orderType types.OrderType) types.Order {
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       trade.Symbol,
			Side:         trade.Side,
			Type:         orderType,
			Quantity:     trade.Quantity,
			Price:        trade.Price,
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		Exchange:         trade.Exchange,
		OrderID:          trade.OrderID,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: trade.Quantity,
		IsWorking:        false,
		CreationTime:     trade.Time,
		UpdateTime:       trade.Time,
	}
}
