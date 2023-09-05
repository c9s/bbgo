package tri

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

//go:generate bash symbols.sh

const ID = "tri"

var log = logrus.WithField("strategy", ID)

var one = fixedpoint.One
var marketOrderProtectiveRatio = fixedpoint.NewFromFloat(0.008)
var balanceBufferRatio = fixedpoint.NewFromFloat(0.005)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Side int

const Buy Side = 1
const Sell Side = -1

func (s Side) String() string {
	return s.SideType().String()
}

func (s Side) SideType() types.SideType {
	if s == 1 {
		return types.SideTypeBuy
	}

	return types.SideTypeSell
}

type PathRank struct {
	Path  *Path
	Ratio float64
}

// backward buy -> buy -> sell
func calculateBackwardRate(p *Path) float64 {
	var ratio = 1.0
	ratio *= p.marketA.calculateRatio(-p.dirA)
	ratio *= p.marketB.calculateRatio(-p.dirB)
	ratio *= p.marketC.calculateRatio(-p.dirC)
	return ratio
}

// calculateForwardRatio
// path: BTCUSDT (0.000044 / 22830.410000) => USDTTWD (0.033220 / 30.101000) => BTCTWD (0.000001 / 687500.000000) <= -> 0.9995899221105569 <- 1.0000373943873788
// 1.0 * 22830 * 30.101000 / 687500.000

// BTCUSDT (0.000044 / 22856.910000) => USDTTWD (0.033217 / 30.104000) => BTCTWD (0.000001 / 688002.100000)
// sell -> rate * 22856
// sell -> rate * 30.104
// buy -> rate / 688002.1
// 1.0000798312
func calculateForwardRatio(p *Path) float64 {
	var ratio = 1.0
	ratio *= p.marketA.calculateRatio(p.dirA)
	ratio *= p.marketB.calculateRatio(p.dirB)
	ratio *= p.marketC.calculateRatio(p.dirC)
	return ratio
}

func adjustOrderQuantityByRate(orders [3]types.SubmitOrder, rate float64) [3]types.SubmitOrder {
	if rate == 1.0 || math.IsNaN(rate) {
		return orders
	}

	for i, o := range orders {
		orders[i].Quantity = o.Quantity.Mul(fixedpoint.NewFromFloat(rate))
	}

	return orders
}

type State struct {
	IOCWinTimes     int     `json:"iocWinningTimes"`
	IOCLossTimes    int     `json:"iocLossTimes"`
	IOCWinningRatio float64 `json:"iocWinningRatio"`
}

type Strategy struct {
	Symbols                    []string                    `json:"symbols"`
	Paths                      [][]string                  `json:"paths"`
	MinSpreadRatio             fixedpoint.Value            `json:"minSpreadRatio"`
	SeparateStream             bool                        `json:"separateStream"`
	Limits                     map[string]fixedpoint.Value `json:"limits"`
	CoolingDownTime            types.Duration              `json:"coolingDownTime"`
	NotifyTrade                bool                        `json:"notifyTrade"`
	ResetPosition              bool                        `json:"resetPosition"`
	MarketOrderProtectiveRatio fixedpoint.Value            `json:"marketOrderProtectiveRatio"`
	IocOrderRatio              fixedpoint.Value            `json:"iocOrderRatio"`
	DryRun                     bool                        `json:"dryRun"`

	markets    map[string]types.Market
	arbMarkets map[string]*ArbMarket
	paths      []*Path

	session *bbgo.ExchangeSession

	activeOrders   *bbgo.ActiveOrderBook
	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector
	Position       *MultiCurrencyPosition `persistence:"position"`
	State          *State                 `persistence:"state"`
	TradeState     *types.TradeStats      `persistence:"trade_stats"`
	sigC           sigchan.Chan
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID + strings.Join(s.Symbols, "-")
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	if !s.SeparateStream {
		for _, symbol := range s.Symbols {
			session.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
				Depth: types.DepthLevelFull,
			})
		}
	}
}

func (s *Strategy) executeOrder(ctx context.Context, order types.SubmitOrder) *types.Order {
	waitTime := 100 * time.Millisecond
	for maxTries := 100; maxTries >= 0; maxTries-- {
		createdOrder, err := s.session.Exchange.SubmitOrder(ctx, order)
		if err != nil {
			log.WithError(err).Errorf("can not submit orders")
			time.Sleep(waitTime)
			waitTime *= 2
			continue
		}

		s.orderStore.Add(*createdOrder)
		s.activeOrders.Add(*createdOrder)
		return createdOrder
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {

	if s.TradeState == nil {
		s.TradeState = types.NewTradeStats("")
	}

	s.Symbols = compileSymbols(s.Symbols)

	if s.MarketOrderProtectiveRatio.IsZero() {
		s.MarketOrderProtectiveRatio = marketOrderProtectiveRatio
	}

	if s.MinSpreadRatio.IsZero() {
		s.MinSpreadRatio = fixedpoint.NewFromFloat(1.002)
	}

	if s.State == nil {
		s.State = &State{}
	}

	s.markets = make(map[string]types.Market)
	s.sigC = sigchan.New(10)

	s.session = session
	s.orderStore = core.NewOrderStore("")
	s.orderStore.AddOrderUpdate = true
	s.orderStore.BindStream(session.UserDataStream)

	s.activeOrders = bbgo.NewActiveOrderBook("")
	s.activeOrders.BindStream(session.UserDataStream)
	s.tradeCollector = core.NewTradeCollector("", nil, s.orderStore)

	for _, symbol := range s.Symbols {
		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market not found: %s", symbol)
		}
		s.markets[symbol] = market
	}
	s.optimizeMarketQuantityPrecision()

	arbMarkets, err := s.buildArbMarkets(session, s.Symbols, s.SeparateStream, s.sigC)
	if err != nil {
		return err
	}

	s.arbMarkets = arbMarkets

	if s.Position == nil {
		s.Position = NewMultiCurrencyPosition(s.markets)
	}

	if s.ResetPosition {
		s.Position = NewMultiCurrencyPosition(s.markets)
	}

	s.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
		s.Position.handleTrade(trade)
	})

	if s.NotifyTrade {
		s.tradeCollector.OnTrade(func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
			bbgo.Notify(trade)
		})
	}

	s.tradeCollector.BindStream(session.UserDataStream)

	for _, market := range s.arbMarkets {
		m := market
		if s.SeparateStream {
			log.Infof("connecting %s market stream...", m.Symbol)
			if err := m.stream.Connect(ctx); err != nil {
				return err
			}
		}
	}

	// build paths
	// rate update and check paths
	for _, pathSymbols := range s.Paths {
		if len(pathSymbols) != 3 {
			return errors.New("a path must contains 3 symbols")
		}

		p := &Path{
			marketA: s.arbMarkets[pathSymbols[0]],
			marketB: s.arbMarkets[pathSymbols[1]],
			marketC: s.arbMarkets[pathSymbols[2]],
		}

		if p.marketA == nil {
			return fmt.Errorf("market object of %s is missing", pathSymbols[0])
		}

		if p.marketB == nil {
			return fmt.Errorf("market object of %s is missing", pathSymbols[1])
		}

		if p.marketC == nil {
			return fmt.Errorf("market object of %s is missing", pathSymbols[2])
		}

		if err := p.solveDirection(); err != nil {
			return err
		}

		s.paths = append(s.paths, p)
	}

	go func() {
		fs := []ratioFunction{calculateForwardRatio, calculateBackwardRate}
		log.Infof("waiting for market prices ready...")
		wait := true
		for wait {
			wait = false
			for _, p := range s.paths {
				if !p.Ready() {
					wait = true
					break
				}
			}
		}

		log.Infof("all markets ready")

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.sigC:
				minRatio := s.MinSpreadRatio.Float64()
				for side, f := range fs {
					ranks := s.calculateRanks(minRatio, f)
					if len(ranks) == 0 {
						break
					}

					forward := side == 0
					bestRank := ranks[0]
					if forward {
						log.Infof("%d paths elected, found best forward path %s profit %.5f%%", len(ranks), bestRank.Path, (bestRank.Ratio-1.0)*100.0)
					} else {
						log.Infof("%d paths elected, found best backward path %s profit %.5f%%", len(ranks), bestRank.Path, (bestRank.Ratio-1.0)*100.0)
					}
					s.executePath(ctx, session, bestRank.Path, bestRank.Ratio, forward)
				}
			}
		}
	}()

	return nil
}

type ratioFunction func(p *Path) float64

func (s *Strategy) checkMinimalOrderQuantity(orders [3]types.SubmitOrder) error {
	for _, order := range orders {
		market := s.arbMarkets[order.Symbol]
		if order.Quantity.Compare(market.market.MinQuantity) < 0 {
			return fmt.Errorf("order quantity is too small: %f < %f", order.Quantity.Float64(), market.market.MinQuantity.Float64())
		}

		if order.Quantity.Mul(order.Price).Compare(market.market.MinNotional) < 0 {
			return fmt.Errorf("order min notional is too small: %f < %f", order.Quantity.Mul(order.Price).Float64(), market.market.MinNotional.Float64())
		}
	}

	return nil
}

func (s *Strategy) optimizeMarketQuantityPrecision() {
	var baseMarkets = make(map[string][]types.Market)
	for _, m := range s.markets {
		baseMarkets[m.BaseCurrency] = append(baseMarkets[m.BaseCurrency], m)
	}

	for _, markets := range baseMarkets {
		var prec = -1
		for _, m := range markets {
			if prec == -1 || m.VolumePrecision < prec {
				prec = m.VolumePrecision
			}
		}

		if prec == -1 {
			continue
		}

		for _, m := range markets {
			m.VolumePrecision = prec
			s.markets[m.Symbol] = m
		}
	}
}

func (s *Strategy) applyBalanceMaxQuantity(balances types.BalanceMap) types.BalanceMap {
	if s.Limits == nil {
		return balances
	}

	for c, b := range balances {
		if limit, ok := s.Limits[c]; ok {
			b.Available = fixedpoint.Min(b.Available, limit)
			balances[c] = b
		}
	}

	return balances
}

func (s *Strategy) addBalanceBuffer(balances types.BalanceMap) (out types.BalanceMap) {
	out = types.BalanceMap{}
	for c, b := range balances {
		ab := b
		ab.Available = ab.Available.Mul(one.Sub(balanceBufferRatio))
		out[c] = ab
	}

	return out
}

func (s *Strategy) toProtectiveMarketOrder(order types.SubmitOrder, ratio fixedpoint.Value) types.SubmitOrder {
	sellRatio := one.Sub(ratio)
	buyRatio := one.Add(ratio)

	switch order.Side {
	case types.SideTypeSell:
		order.Price = order.Price.Mul(sellRatio)

	case types.SideTypeBuy:
		order.Price = order.Price.Mul(buyRatio)
	}

	return order
}

func (s *Strategy) toProtectiveMarketOrders(orders [3]types.SubmitOrder, ratio fixedpoint.Value) [3]types.SubmitOrder {
	sellRatio := one.Sub(ratio)
	buyRatio := one.Add(ratio)
	for i, order := range orders {
		switch order.Side {
		case types.SideTypeSell:
			order.Price = order.Price.Mul(sellRatio)

		case types.SideTypeBuy:
			order.Price = order.Price.Mul(buyRatio)
		}

		// order.Quantity = order.Market.TruncateQuantity(order.Quantity)
		// order.Type = types.OrderTypeMarket
		orders[i] = order
	}

	return orders
}

func (s *Strategy) executePath(ctx context.Context, session *bbgo.ExchangeSession, p *Path, ratio float64, dir bool) {
	balances := session.Account.Balances()
	balances = s.addBalanceBuffer(balances)
	balances = s.applyBalanceMaxQuantity(balances)

	var orders [3]types.SubmitOrder
	if dir {
		orders = p.newOrders(balances, 1)
	} else {
		orders = p.newOrders(balances, -1)
	}

	if err := s.checkMinimalOrderQuantity(orders); err != nil {
		log.WithError(err).Warnf("order quantity too small, skip")
		return
	}

	if s.DryRun {
		logSubmitOrders(orders)
		return
	}

	createdOrders, err := s.iocOrderExecution(ctx, session, orders, ratio)
	if err != nil {
		log.WithError(err).Errorf("order execute error")
		return
	}

	if len(createdOrders) == 0 {
		return
	}

	log.Info(s.Position.String())

	profits := s.Position.CollectProfits()
	profitInUSD := fixedpoint.Zero
	for _, profit := range profits {
		bbgo.Notify(&profit)
		log.Info(profit.PlainText())

		profitInUSD = profitInUSD.Add(profit.ProfitInUSD)

		// FIXME:
		// s.TradeState.Add(&profit)
	}

	notifyUsdPnL(profitInUSD)

	log.Info(s.TradeState.BriefString())

	bbgo.Sync(ctx, s)

	if s.CoolingDownTime > 0 {
		log.Infof("cooling down for %s", s.CoolingDownTime.Duration().String())
		time.Sleep(s.CoolingDownTime.Duration())
	}
}

func notifyUsdPnL(profit fixedpoint.Value) {
	var title = "Triangular Sum PnL ~= "
	title += style.PnLEmojiSimple(profit) + " "
	title += style.PnLSignString(profit) + " USD"
	bbgo.Notify(title)
}

func (s *Strategy) iocOrderExecution(ctx context.Context, session *bbgo.ExchangeSession, orders [3]types.SubmitOrder, ratio float64) (types.OrderSlice, error) {
	service, ok := session.Exchange.(types.ExchangeOrderQueryService)
	if !ok {
		return nil, errors.New("exchange does not support ExchangeOrderQueryService")
	}

	var filledQuantity = fixedpoint.Zero

	// Change the first order to IOC
	orders[0].Type = types.OrderTypeLimit
	orders[0].TimeInForce = types.TimeInForceIOC

	var originalOrders [3]types.SubmitOrder
	originalOrders[0] = orders[0]
	originalOrders[1] = orders[1]
	originalOrders[2] = orders[2]
	logSubmitOrders(orders)

	if !s.IocOrderRatio.IsZero() {
		orders[0] = s.toProtectiveMarketOrder(orders[0], s.IocOrderRatio)
	}

	iocOrder := s.executeOrder(ctx, orders[0])
	if iocOrder == nil {
		return nil, errors.New("ioc order submit error")
	}

	iocOrderC := make(chan types.Order, 2)
	defer func() {
		close(iocOrderC)
	}()

	go func() {
		o, err := s.waitWebSocketOrderDone(ctx, iocOrder.OrderID, 300*time.Millisecond)
		if err != nil {
			// log.WithError(err).Errorf("ioc order wait error")
			return
		} else if o != nil {
			select {
			case iocOrderC <- *o:
			default:
			}
		}
	}()

	go func() {
		o, err := waitForOrderFilled(ctx, service, *iocOrder, 3*time.Second)
		if err != nil {
			log.WithError(err).Errorf("ioc order restful wait error")
			return
		} else if o != nil {
			select {
			case iocOrderC <- *o:
			default:
			}
		}
	}()

	o := <-iocOrderC

	filledQuantity = o.ExecutedQuantity

	if filledQuantity.IsZero() {
		s.State.IOCLossTimes++

		// we didn't get filled
		log.Infof("%s %s IOC order did not get filled, skip: %+v", o.Symbol, o.Side, o)
		return nil, nil
	}

	filledRatio := filledQuantity.Div(iocOrder.Quantity)
	bbgo.Notify("%s %s IOC order got filled %f/%f (%s)", iocOrder.Symbol, iocOrder.Side, filledQuantity.Float64(), iocOrder.Quantity.Float64(), filledRatio.Percentage())
	log.Infof("%s %s IOC order got filled %f/%f", iocOrder.Symbol, iocOrder.Side, filledQuantity.Float64(), iocOrder.Quantity.Float64())

	orders[1].Quantity = orders[1].Quantity.Mul(filledRatio)
	orders[2].Quantity = orders[2].Quantity.Mul(filledRatio)

	if orders[1].Quantity.Compare(orders[1].Market.MinQuantity) <= 0 {
		log.Warnf("order #2 quantity %f is less than min quantity %f, skip", orders[1].Quantity.Float64(), orders[1].Market.MinQuantity.Float64())
		return nil, nil
	}

	if orders[2].Quantity.Compare(orders[2].Market.MinQuantity) <= 0 {
		log.Warnf("order #3 quantity %f is less than min quantity %f, skip", orders[2].Quantity.Float64(), orders[2].Market.MinQuantity.Float64())
		return nil, nil
	}

	orders[1] = s.toProtectiveMarketOrder(orders[1], s.MarketOrderProtectiveRatio)
	orders[2] = s.toProtectiveMarketOrder(orders[2], s.MarketOrderProtectiveRatio)

	var orderC = make(chan types.Order, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		o := s.executeOrder(ctx, orders[1])
		orderC <- *o
		wg.Done()
	}()

	go func() {
		o := s.executeOrder(ctx, orders[2])
		orderC <- *o
		wg.Done()
	}()

	wg.Wait()

	var createdOrders = make(types.OrderSlice, 3)
	createdOrders[0] = *iocOrder
	createdOrders[1] = <-orderC
	createdOrders[2] = <-orderC
	close(orderC)

	orderTrades, updatedOrders, err := s.waitOrdersAndCollectTrades(ctx, service, createdOrders)
	if err != nil {
		log.WithError(err).Errorf("trade collecting error")
	} else {
		for i, order := range updatedOrders {
			trades, hasTrades := orderTrades[order.OrderID]
			if !hasTrades {
				continue
			}
			averagePrice := tradeAveragePrice(trades, order.OrderID)
			updatedOrders[i].AveragePrice = averagePrice

			if market, hasMarket := s.markets[order.Symbol]; hasMarket {
				updatedOrders[i].Market = market
			}

			for _, originalOrder := range originalOrders {
				if originalOrder.Symbol == updatedOrders[i].Symbol {
					updatedOrders[i].Price = originalOrder.Price
				}
			}
		}
		s.analyzeOrders(updatedOrders)
	}

	// update ioc winning ratio
	s.State.IOCWinTimes++
	if s.State.IOCLossTimes == 0 {
		s.State.IOCWinningRatio = 999.0
	} else {
		s.State.IOCWinningRatio = float64(s.State.IOCWinTimes) / float64(s.State.IOCLossTimes)
	}

	log.Infof("ioc winning ratio update: %f", s.State.IOCWinningRatio)

	return createdOrders, nil
}

func (s *Strategy) waitWebSocketOrderDone(ctx context.Context, orderID uint64, timeoutDuration time.Duration) (*types.Order, error) {
	prof := util.StartTimeProfile("waitWebSocketOrderDone")
	defer prof.StopAndLog(log.Infof)

	if order, ok := s.orderStore.Get(orderID); ok {
		if order.Status == types.OrderStatusFilled || order.Status == types.OrderStatusCanceled {
			return &order, nil
		}
	}

	timeoutC := time.After(timeoutDuration)
	for {
		select {

		case <-ctx.Done():
			return nil, ctx.Err()

		case <-timeoutC:
			return nil, fmt.Errorf("order wait time timeout %s", timeoutDuration)

		case order := <-s.orderStore.C:
			if orderID == order.OrderID && (order.Status == types.OrderStatusFilled || order.Status == types.OrderStatusCanceled) {
				return &order, nil
			}
		}
	}
}

func (s *Strategy) waitOrdersAndCollectTrades(ctx context.Context, service types.ExchangeOrderQueryService, createdOrders types.OrderSlice) (map[uint64][]types.Trade, types.OrderSlice, error) {
	var err error
	var orderTrades = make(map[uint64][]types.Trade)
	var updatedOrders types.OrderSlice
	for _, o := range createdOrders {
		updatedOrder, err2 := waitForOrderFilled(ctx, service, o, time.Second)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		trades, err3 := service.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  o.Symbol,
			OrderID: strconv.FormatUint(o.OrderID, 10),
		})
		if err3 != nil {
			err = multierr.Append(err, err3)
			continue
		}

		for _, t := range trades {
			s.tradeCollector.ProcessTrade(t)
		}

		orderTrades[o.OrderID] = trades
		updatedOrders = append(updatedOrders, *updatedOrder)
	}

	/*
	 */
	return orderTrades, updatedOrders, nil
}

func (s *Strategy) analyzeOrders(orders types.OrderSlice) {
	sort.Slice(orders, func(i, j int) bool {
		// o1 < o2 -- earlier first
		return orders[i].CreationTime.Before(orders[i].CreationTime.Time())
	})

	log.Infof("ANALYZING ORDERS (Earlier First)")
	for i, o := range orders {
		in, inCurrency := o.In()
		out, outCurrency := o.Out()
		log.Infof("#%d %s IN %f %s -> OUT %f %s", i, o.String(), in.Float64(), inCurrency, out.Float64(), outCurrency)
	}

	for _, o := range orders {
		switch o.Side {
		case types.SideTypeSell:
			price := o.Price
			priceDiff := o.AveragePrice.Sub(price)
			slippage := priceDiff.Div(price)
			log.Infof("%-8s %-4s %-10s AVG PRICE %f PRICE %f Q %f SLIPPAGE %.3f%%", o.Symbol, o.Side, o.Type, o.AveragePrice.Float64(), price.Float64(), o.Quantity.Float64(), slippage.Float64()*100.0)

		case types.SideTypeBuy:
			price := o.Price
			priceDiff := price.Sub(o.AveragePrice)
			slippage := priceDiff.Div(price)
			log.Infof("%-8s %-4s %-10s AVG PRICE %f PRICE %f Q %f SLIPPAGE %.3f%%", o.Symbol, o.Side, o.Type, o.AveragePrice.Float64(), price.Float64(), o.Quantity.Float64(), slippage.Float64()*100.0)
		}
	}
}

func (s *Strategy) buildArbMarkets(session *bbgo.ExchangeSession, symbols []string, separateStream bool, sigC sigchan.Chan) (map[string]*ArbMarket, error) {
	markets := make(map[string]*ArbMarket)
	// build market object
	for _, symbol := range symbols {
		market, ok := s.markets[symbol]
		if !ok {
			return nil, fmt.Errorf("market not found: %s", symbol)
		}

		m := &ArbMarket{
			Symbol:        symbol,
			market:        market,
			BaseCurrency:  market.BaseCurrency,
			QuoteCurrency: market.QuoteCurrency,
			sigC:          sigC,
		}

		if separateStream {
			stream := session.Exchange.NewStream()
			stream.SetPublicOnly()
			stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
				Depth: types.DepthLevelFull,
				Speed: types.SpeedHigh,
			})

			book := types.NewStreamBook(symbol)
			priceUpdater := func(_ types.SliceOrderBook) {
				bestAsk, bestBid, _ := book.BestBidAndAsk()
				if bestAsk.Equals(m.bestAsk) && bestBid.Equals(m.bestBid) {
					return
				}

				m.bestBid = bestBid
				m.bestAsk = bestAsk
				m.updateRate()
			}
			book.OnUpdate(priceUpdater)
			book.OnSnapshot(priceUpdater)
			book.BindStream(stream)

			stream.OnDisconnect(func() {
				// reset price and volume
				m.bestBid = types.PriceVolume{}
				m.bestAsk = types.PriceVolume{}
			})

			m.book = book
			m.stream = stream
		} else {
			book, _ := session.OrderBook(symbol)
			priceUpdater := func(_ types.SliceOrderBook) {
				bestAsk, bestBid, _ := book.BestBidAndAsk()
				if bestAsk.Equals(m.bestAsk) && bestBid.Equals(m.bestBid) {
					return
				}

				m.bestBid = bestBid
				m.bestAsk = bestAsk
				m.updateRate()
			}
			book.OnUpdate(priceUpdater)
			book.OnSnapshot(priceUpdater)

			m.book = book
			m.stream = session.MarketDataStream
		}

		markets[symbol] = m
	}

	return markets, nil
}

func (s *Strategy) calculateRanks(minRatio float64, method func(p *Path) float64) []PathRank {
	ranks := make([]PathRank, 0, len(s.paths))

	// ranking paths here
	for _, path := range s.paths {
		ratio := method(path)
		if ratio < minRatio {
			continue
		}

		p := path
		ranks = append(ranks, PathRank{Path: p, Ratio: ratio})
	}

	// sort and pick up the top rank path
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].Ratio > ranks[j].Ratio
	})

	return ranks
}

func waitForOrderFilled(ctx context.Context, ex types.ExchangeOrderQueryService, order types.Order, timeout time.Duration) (*types.Order, error) {
	prof := util.StartTimeProfile("waitForOrderFilled")
	defer prof.StopAndLog(log.Infof)

	timeoutC := time.After(timeout)

	for {
		select {
		case <-timeoutC:
			return nil, fmt.Errorf("order wait timeout %s", timeout)

		default:
			p := util.StartTimeProfile("queryOrder")
			remoteOrder, err2 := ex.QueryOrder(ctx, types.OrderQuery{
				Symbol:  order.Symbol,
				OrderID: strconv.FormatUint(order.OrderID, 10),
			})
			p.StopAndLog(log.Infof)

			if err2 != nil {
				log.WithError(err2).Errorf("order query error")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			switch remoteOrder.Status {
			case types.OrderStatusFilled, types.OrderStatusCanceled:
				return remoteOrder, nil
			default:
				log.Infof("WAITING: %s", remoteOrder.String())
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}

func tradeAveragePrice(trades []types.Trade, orderID uint64) fixedpoint.Value {
	totalAmount := fixedpoint.Zero
	totalQuantity := fixedpoint.Zero
	for _, trade := range trades {
		if trade.OrderID != orderID {
			continue
		}

		totalAmount = totalAmount.Add(trade.Price.Mul(trade.Quantity))
		totalQuantity = totalQuantity.Add(trade.Quantity)
	}

	return totalAmount.Div(totalQuantity)
}
