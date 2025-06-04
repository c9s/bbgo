package xmaker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PositionExposure struct {
	symbol string

	// net = net position
	// pending = covered position
	net, pending fixedpoint.MutexValue
}

func newPositionExposure(symbol string) *PositionExposure {
	return &PositionExposure{
		symbol: symbol,
	}
}

func (m *PositionExposure) Open(delta fixedpoint.Value) {
	m.net.Add(delta)

	log.Infof(
		"%s opened:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) Cover(delta fixedpoint.Value) {
	m.pending.Add(delta)

	log.Infof(
		"%s covered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) Close(delta fixedpoint.Value) {
	m.pending.Add(delta)
	m.net.Add(delta)

	log.Infof(
		"%s closed:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) GetUncovered() fixedpoint.Value {
	netPosition := m.net.Get()
	coveredPosition := m.pending.Get()
	uncoverPosition := netPosition.Sub(coveredPosition)

	log.Infof(
		"%s netPosition:%v coveredPosition: %v uncoverPosition: %v",
		m.symbol,
		netPosition,
		coveredPosition,
		uncoverPosition,
	)

	return uncoverPosition
}

// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
// SourceSymbol could be something like binance.BTCUSDT
// FiatSymbol could be something like max.USDTTWD
type SyntheticHedge struct {
	// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
	Enabled bool `json:"enabled"`

	SourceSymbol       string           `json:"sourceSymbol"`
	SourceDepthInQuote fixedpoint.Value `json:"sourceDepthInQuote"`

	FiatSymbol       string           `json:"fiatSymbol"`
	FiatDepthInQuote fixedpoint.Value `json:"fiatDepthInQuote"`

	HedgeInterval types.Duration `json:"hedgeInterval"`

	sourceMarket, fiatMarket *HedgeMarket

	syntheticTradeId uint64

	logger logrus.FieldLogger

	mu sync.Mutex
}

// InitializeAndBind
// not a good way to initialize the synthetic hedge with the strategy instance
// but we need to build the trade collector to update the profit and position
func (s *SyntheticHedge) InitializeAndBind(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
	strategy *Strategy,
) error {
	if !s.Enabled {
		return nil
	}

	// Initialize the synthetic quote
	if s.SourceSymbol == "" || s.FiatSymbol == "" {
		return fmt.Errorf("sourceSymbol and fiatSymbol must be set")
	}

	s.logger = log.WithFields(logrus.Fields{
		"component": "synthetic_hedge",
	})

	var err error

	sourceSession, sourceMarket, err := parseSymbolSelector(s.SourceSymbol, sessions)
	if err != nil {
		return err
	}

	fiatSession, fiatMarket, err := parseSymbolSelector(s.FiatSymbol, sessions)
	if err != nil {
		return err
	}

	s.sourceMarket = newHedgeMarket(sourceSession, sourceMarket, s.SourceDepthInQuote)
	s.fiatMarket = newHedgeMarket(fiatSession, fiatMarket, s.FiatDepthInQuote)

	// when receiving trades from the source session,
	// mock a trade with the quote amount and add to the fiat position
	//
	// hedge flow:
	//   maker trade -> update source market position delta
	//   source market position delta -> convert to fiat trade -> update fiat market position delta
	s.sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		price := fixedpoint.Zero

		// 1) get the fiat book price from the snapshot when possible
		bid, ask := s.fiatMarket.getQuotePrice()

		// 2) build up fiat position from the trade quote quantity
		fiatQuantity := trade.QuoteQuantity
		fiatDelta := fiatQuantity

		// reverse side because we assume source's quote currency is the fiat market's base currency here
		fiatSide := trade.Side.Reverse()

		switch trade.Side {
		case types.SideTypeBuy:
			price = ask
			fiatDelta = fiatQuantity.Neg()
		case types.SideTypeSell:
			price = bid
			fiatDelta = fiatQuantity
		default:
			return
		}

		// 3) convert source trade into fiat trade as the average cost of the fiat market position
		// This assumes source's quote currency is the fiat market's base currency
		fiatTrade := s.fiatMarket.newMockTrade(fiatSide, price, fiatQuantity, trade.Time.Time())
		if profit, netProfit, madeProfit := s.fiatMarket.position.AddTrade(fiatTrade); madeProfit {
			// TODO: record the profits in somewhere?
			_ = profit
			_ = netProfit
		}

		// add position delta to let the fiat market hedge the position
		s.fiatMarket.positionDeltaC <- fiatDelta
	})

	s.fiatMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		avgCost := s.sourceMarket.position.AverageCost

		// convert the trade quantity to the source market's base currency quantity
		// calculate how much base quantity we can close the source position
		baseQuantity := fixedpoint.Zero
		if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
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
		if profit, netProfit, madeProfit := s.sourceMarket.position.AddTrade(mockSourceTrade); madeProfit {
			_ = profit
			_ = netProfit
		}

		// close the maker position
		// create a mock trade for closing the maker position
		// This assumes the source market's quote currency is the fiat market's base currency
		price := fixedpoint.Zero
		if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
			price = avgCost.Mul(trade.Price)
		} else {
			price = avgCost.Div(trade.Price)
		}

		side := trade.Side
		tradeId := atomic.AddUint64(&s.syntheticTradeId, 1)
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
			Fee:           trade.Fee,         // apply trade fee when possible
			FeeCurrency:   trade.FeeCurrency, // apply trade fee when possible
		}
		s.logger.Infof("synthetic trade created: %+v, average cost: %f", syntheticTrade, avgCost.Float64())
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
	hedgeDelta fixedpoint.Value,
) error {
	if hedgeDelta.IsZero() {
		return nil
	}

	s.logger.Infof("synthetic hedging with delta: %f", hedgeDelta.Float64())
	s.sourceMarket.positionDeltaC <- hedgeDelta
	return nil
}

func (s *SyntheticHedge) GetQuotePrices() (fixedpoint.Value, fixedpoint.Value, bool) {
	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	bid, ask := s.sourceMarket.getQuotePrice()
	bid2, ask2 := s.fiatMarket.getQuotePrice()

	if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
		bid = bid.Mul(bid2)
		ask = ask.Mul(ask2)
		return bid, ask, bid.Sign() > 0 && ask.Sign() > 0
	}

	if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.QuoteCurrency {
		bid = bid.Div(bid2)
		ask = ask.Div(ask2)
		return bid, ask, bid.Sign() > 0 && ask.Sign() > 0
	}

	return fixedpoint.Zero, fixedpoint.Zero, false
}

func (s *SyntheticHedge) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fmt.Errorf("sourceMarket and fiatMarket must be initialized")
	}

	interval := s.HedgeInterval.Duration()
	if interval == 0 {
		interval = 2 * time.Second // default interval
	}

	if err := s.sourceMarket.start(ctx, interval); err != nil {
		return err
	}

	if err := s.fiatMarket.start(ctx, interval); err != nil {
		return err
	}

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
