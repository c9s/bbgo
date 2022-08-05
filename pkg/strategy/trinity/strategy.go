package trinity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "trinity"

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

type ArbMarket struct {
	Symbol                      string
	BaseCurrency, QuoteCurrency string
	market                      types.Market

	stream            types.Stream
	book              *types.StreamOrderBook
	bestBid, bestAsk  types.PriceVolume
	buyRate, sellRate float64
	sigC              sigchan.Chan
}

func (m *ArbMarket) String() string {
	return fmt.Sprintf("%s (%f / %f)", m.Symbol, m.buyRate, m.sellRate)
}

func (m *ArbMarket) getInitialBalance(balances types.BalanceMap, dir int) (fixedpoint.Value, string) {
	if dir == 1 { // sell 1 BTC -> 19000 USDT
		b, ok := balances[m.BaseCurrency]
		if !ok {
			return fixedpoint.Zero, m.BaseCurrency
		}

		return b.Available, m.BaseCurrency
	} else if dir == -1 {
		b, ok := balances[m.QuoteCurrency]
		if !ok {
			return fixedpoint.Zero, m.QuoteCurrency
		}

		return b.Available, m.QuoteCurrency
	}

	return fixedpoint.Zero, ""
}

func (m *ArbMarket) calculateRatio(dir int) float64 {
	if dir == 1 { // direct 1 = sell
		if m.bestBid.Volume.Compare(m.market.MinQuantity) < 0 {
			return 0.0
		}

		return m.sellRate
	} else if dir == -1 {
		if m.bestAsk.Volume.Compare(m.market.MinQuantity) < 0 {
			return 0.0
		}

		return m.buyRate
	}

	return 0.0
}

func (m *ArbMarket) updateRate() {
	m.buyRate = 1.0 / m.bestAsk.Price.Float64()
	m.sellRate = m.bestBid.Price.Float64()

	if m.bestBid.Volume.Compare(m.market.MinQuantity) < 0 && m.bestAsk.Volume.Compare(m.market.MinQuantity) < 0 {
		return
	}

	m.sigC.Emit()
}

func (m *ArbMarket) newOrder(dir int, transitingQuantity float64) (types.SubmitOrder, float64) {
	if dir == 1 { // sell ETH -> BTC, sell USDT -> TWD
		q, r := fitQuantityByBase(m.bestBid.Volume.Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		fq = m.market.TruncateQuantity(fq)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fq,
			Price:    m.bestBid.Price,
			Market:   m.market,
		}, r
	} else if dir == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(m.bestAsk.Price.Float64(), m.bestAsk.Volume.Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		fq = m.market.TruncateQuantity(fq)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fq,
			Price:    m.bestAsk.Price,
			Market:   m.market,
		}, r
	}

	return types.SubmitOrder{}, 0.0
}

func buildArbMarkets(session *bbgo.ExchangeSession, symbols []string, separateStream bool, sigC sigchan.Chan) (map[string]*ArbMarket, error) {
	markets := make(map[string]*ArbMarket)
	// build market object
	for _, symbol := range symbols {
		market, ok := session.Market(symbol)
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
				m.bestAsk, m.bestBid, _ = book.BestBidAndAsk()
				m.updateRate()
			}
			book.OnUpdate(priceUpdater)
			book.OnSnapshot(priceUpdater)
			book.BindStream(stream)

			m.book = book
			m.stream = stream
		} else {
			book, _ := session.OrderBook(symbol)
			priceUpdater := func(_ types.SliceOrderBook) {
				m.bestAsk, m.bestBid, _ = book.BestBidAndAsk()
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

type Path struct {
	marketA, marketB, marketC *ArbMarket
	dirA, dirB, dirC          int
}

func (p *Path) solveDirection() error {
	// check if we should reverse the rate
	// ETHUSDT -> ETHBTC
	if p.marketA.QuoteCurrency == p.marketB.BaseCurrency || p.marketA.QuoteCurrency == p.marketB.QuoteCurrency {
		p.dirA = 1
	} else if p.marketA.BaseCurrency == p.marketB.BaseCurrency || p.marketA.BaseCurrency == p.marketB.QuoteCurrency {
		p.dirA = -1
	} else {
		return fmt.Errorf("marketA and marketB is not related")
	}

	if p.marketB.QuoteCurrency == p.marketC.BaseCurrency || p.marketB.QuoteCurrency == p.marketC.QuoteCurrency {
		p.dirB = 1
	} else if p.marketB.BaseCurrency == p.marketC.BaseCurrency || p.marketB.BaseCurrency == p.marketC.QuoteCurrency {
		p.dirB = -1
	} else {
		return fmt.Errorf("marketB and marketC is not related")
	}

	if p.marketC.QuoteCurrency == p.marketA.BaseCurrency || p.marketC.QuoteCurrency == p.marketA.QuoteCurrency {
		p.dirC = 1
	} else if p.marketC.BaseCurrency == p.marketA.BaseCurrency || p.marketC.BaseCurrency == p.marketA.QuoteCurrency {
		p.dirC = -1
	} else {
		return fmt.Errorf("marketC and marketA is not related")
	}

	return nil
}

func (p *Path) Ready() bool {
	return !(p.marketA.bestAsk.Price.IsZero() || p.marketA.bestBid.Price.IsZero() ||
		p.marketB.bestAsk.Price.IsZero() || p.marketB.bestBid.Price.IsZero() ||
		p.marketC.bestAsk.Price.IsZero() || p.marketC.bestBid.Price.IsZero())
}

func (p *Path) String() string {
	return p.marketA.String() + " " + toDirection(p.dirA) + " " + p.marketB.String() + " " + toDirection(p.dirB) + " " + p.marketC.String() + " " + toDirection(p.dirC) +
		" -> " + strconv.FormatFloat(calculateForwardRatio(p), 'f', -1, 64) +
		" <- " + strconv.FormatFloat(calculateBackwardRate(p), 'f', -1, 64)
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

func (p *Path) newForwardOrders(balances types.BalanceMap) []types.SubmitOrder {
	var transitingQuantity float64
	var transitingCurrency string
	var orders []types.SubmitOrder

	initialBalance, transitingCurrency := p.marketA.getInitialBalance(balances, p.dirA)
	orderA, _ := p.marketA.newOrder(p.dirB, initialBalance.Float64())
	orders = append(orders, orderA)

	q, c := orderA.Out()
	transitingQuantity, transitingCurrency = q.Float64(), c
	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)

	// orderB
	orderB, rateB := p.marketB.newOrder(p.dirB, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateB)

	q, c = orderB.Out()
	transitingQuantity, transitingCurrency = q.Float64(), c
	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)

	orders = append(orders, orderB)

	orderC, rateC := p.marketC.newOrder(p.dirC, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateC)

	q, c = orderC.Out()
	log.Infof("FINAL QUANTITY %f %s", q.Float64(), c)

	orders = append(orders, orderC)

	log.Infof("FINAL ORDERS:")
	logSubmitOrders(orders)
	return orders
}

func adjustOrderQuantityByRate(orders []types.SubmitOrder, rate float64) []types.SubmitOrder {
	if rate == 1.0 {
		return orders
	}

	for i, o := range orders {
		orders[i].Quantity = o.Quantity.Mul(fixedpoint.NewFromFloat(rate))
	}

	return orders
}

func toDirection(d int) string {
	switch d {
	case -1:
		return "<="
	case 1:
		return "=>"
	case 0:
		return "(undefined)"

	default:
		return "(undefined)"
	}
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

	markets    map[string]types.Market
	arbMarkets map[string]*ArbMarket
	paths      []*Path

	session *bbgo.ExchangeSession

	activeOrders   *bbgo.ActiveOrderBook
	orderStore     *bbgo.OrderStore
	tradeCollector *bbgo.TradeCollector
	Position       *MultiCurrencyPosition `persistence:"position"`
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

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.MarketOrderProtectiveRatio.IsZero() {
		s.MarketOrderProtectiveRatio = marketOrderProtectiveRatio
	}

	if s.MinSpreadRatio.IsZero() {
		s.MinSpreadRatio = fixedpoint.NewFromFloat(1.002)
	}

	s.markets = make(map[string]types.Market)
	s.sigC = sigchan.New(10)

	s.session = session
	s.orderStore = bbgo.NewOrderStore("")
	s.orderStore.BindStream(session.UserDataStream)

	s.activeOrders = bbgo.NewActiveOrderBook("")
	s.activeOrders.BindStream(session.UserDataStream)
	s.tradeCollector = bbgo.NewTradeCollector("", nil, s.orderStore)

	for _, symbol := range s.Symbols {
		market, ok := session.Market(symbol)
		if !ok {
			return fmt.Errorf("market not found: %s", symbol)
		}
		s.markets[symbol] = market
	}
	s.optimizeMarketQuantityPrecision()

	arbMarkets, err := buildArbMarkets(session, s.Symbols, s.SeparateStream, s.sigC)
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
	for _, symbols := range s.Paths {
		if len(symbols) != 3 {
			return errors.New("a path must contains 3 symbols")
		}

		p := &Path{
			marketA: s.arbMarkets[symbols[0]],
			marketB: s.arbMarkets[symbols[1]],
			marketC: s.arbMarkets[symbols[2]],
		}

		if p.marketA == nil {
			return fmt.Errorf("market object of %s is missing", symbols[0])
		}

		if p.marketB == nil {
			return fmt.Errorf("market object of %s is missing", symbols[1])
		}

		if p.marketC == nil {
			return fmt.Errorf("market object of %s is missing", symbols[2])
		}

		if err := p.solveDirection(); err != nil {
			return err
		}

		s.paths = append(s.paths, p)
	}

	go func() {
		// fs := []ratioFunction{calculateForwardRatio, calculateBackwardRate}
		fs := []ratioFunction{calculateForwardRatio}
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
					s.executePath(ctx, session, ranks[0].Path, side == 0)
				}
			}
		}
	}()

	return nil
}

type ratioFunction func(p *Path) float64

func (s *Strategy) checkMinimalOrderQuantity(orders []types.SubmitOrder) error {
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
		var prec = 8
		for _, m := range markets {
			if m.VolumePrecision < prec {
				prec = m.VolumePrecision
			}
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

func (s *Strategy) toProtectiveMarketOrders(orders []types.SubmitOrder) []types.SubmitOrder {
	var out []types.SubmitOrder

	for _, order := range orders {
		switch order.Side {
		case types.SideTypeSell:
			order.Price = order.Price.Mul(one.Sub(s.MarketOrderProtectiveRatio))

		case types.SideTypeBuy:
			order.Price = order.Price.Mul(one.Add(s.MarketOrderProtectiveRatio))
		}

		out = append(out, order)
	}

	return out
}

func (s *Strategy) executePath(ctx context.Context, session *bbgo.ExchangeSession, p *Path, dir bool) {
	log.Infof("executing path: %+v", p)
	balances := session.Account.Balances()
	balances = s.addBalanceBuffer(balances)
	balances = s.applyBalanceMaxQuantity(balances)

	var orders []types.SubmitOrder
	if dir {
		orders = p.newForwardOrders(balances)
	} else {
		return
	}

	if len(orders) == 0 {
		return
	}

	if err := s.checkMinimalOrderQuantity(orders); err != nil {
		log.WithError(err).Warnf("minimalOrderQuantity error")
		return
	}

	// show orders
	logSubmitOrders(orders)
	orders = s.toProtectiveMarketOrders(orders)

	log.Infof("adjusted to protective market orders:")
	logSubmitOrders(orders)

	createdOrders, err := session.Exchange.SubmitOrders(ctx, orders...)
	if err != nil {
		log.WithError(err).Errorf("can not submit orders")
	}
	s.orderStore.Add(createdOrders...)
	s.activeOrders.Add(createdOrders...)

	timeoutDuration := 500 * time.Millisecond
	timeout := time.After(timeoutDuration)
	wait := true
	for wait && s.activeOrders.NumOfOrders() > 0 {
		select {
		case <-ctx.Done():
			wait = false
			log.WithError(ctx.Err()).Warnf("context done")
			break
		case <-timeout:
			wait = false
			log.Warnf("order wait time timeout %s", timeoutDuration)
			break

		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if service, ok := session.Exchange.(types.ExchangeOrderQueryService); ok {
		updatedOrders, allFilled := waitForAllOrdersFilled(ctx, service, createdOrders, 20)
		if allFilled {
			log.Infof("all orders are filled!")
		}
		createdOrders = updatedOrders

		trades, err := collectOrdersTrades(ctx, service, updatedOrders)
		if err != nil {
			log.WithError(err).Errorf("failed to query order trades")
		} else {
			for _, t := range trades {
				s.tradeCollector.ProcessTrade(t)
			}
		}
	} else {
		// wait for trades
		time.Sleep(200 * time.Millisecond)
	}

	s.tradeCollector.Process()

	log.Infof("final position: %s", s.Position.String())

	profits := s.Position.CollectProfits()
	for _, profit := range profits {
		bbgo.Notify(&profit)
	}

	if s.CoolingDownTime > 0 {
		log.Infof("cooling down for %s", s.CoolingDownTime.Duration().String())
		time.Sleep(s.CoolingDownTime.Duration())
	}
}

func (s *Strategy) calculateRanks(minRatio float64, method func(p *Path) float64) []PathRank {
	ranks := make([]PathRank, 0, len(s.paths))

	// ranking paths here
	for i, path := range s.paths {
		_ = i
		ratio := method(path)
		if ratio < minRatio {
			continue
		}

		p := path
		log.Infof("adding path #%d: ratio: %f path: %+v", i, ratio, p)
		ranks = append(ranks, PathRank{Path: p, Ratio: ratio})
	}

	// sort and pick up the top rank path
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].Ratio > ranks[j].Ratio
	})

	return ranks
}

func collectOrdersTrades(ctx context.Context, ex types.ExchangeOrderQueryService, createdOrders types.OrderSlice) ([]types.Trade, error) {
	var ordersTrades []types.Trade
	var err2 error
	for _, o := range createdOrders {
		trades, err := ex.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:  o.Symbol,
			OrderID: strconv.FormatUint(o.OrderID, 10),
		})

		if err != nil {
			err2 = err
			continue
		}

		ordersTrades = append(ordersTrades, trades...)
	}

	return ordersTrades, err2
}

func waitForAllOrdersFilled(ctx context.Context, ex types.ExchangeOrderQueryService, orders types.OrderSlice, maxTries int) (types.OrderSlice, bool) {
	log.Infof("query order service to ensure orders are filled")
	allFilled := false
	for ; !allFilled && maxTries > 0; maxTries-- {
		allFilled = true
		for i, o := range orders {
			remoteOrder, err2 := ex.QueryOrder(ctx, types.OrderQuery{
				Symbol:  o.Symbol,
				OrderID: strconv.FormatUint(o.OrderID, 10),
			})

			if err2 != nil {
				log.WithError(err2).Errorf("order query error")
				continue
			}

			orders[i] = *remoteOrder

			if remoteOrder.Status != types.OrderStatusFilled {
				log.Infof(remoteOrder.String())
				allFilled = false
			}

			time.Sleep(200 * time.Millisecond)
		}
	}
	return orders, allFilled
}

func fitQuantityByBase(quantity, balance float64) (float64, float64) {
	q := math.Min(quantity, balance)
	r := q / balance
	return q, r
}

// 1620 x 2 , quote balance = 1000 => rate = 1000/(1620*2) = 0.3086419753, quantity = 0.61728395
func fitQuantityByQuote(price, quantity, quoteBalance float64) (float64, float64) {
	quote := quantity * price
	minQuote := math.Min(quote, quoteBalance)
	q := minQuote / price
	r := minQuote / quoteBalance
	return q, r
}

func logSubmitOrders(orders []types.SubmitOrder) {
	for i, order := range orders {
		in, inCurrency := order.In()
		out, outCurrency := order.Out()
		log.Infof("SUBMIT ORDER #%d: %+v IN: %f %s => OUT: %f %s", i, order.String(), in.Float64(), inCurrency, out.Float64(), outCurrency)
	}
}
