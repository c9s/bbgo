package trinity

import (
	"context"
	"errors"
	"fmt"
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
		var prec = 9
		for _, m := range markets {
			if m.VolumePrecision < prec {
				prec = m.VolumePrecision
			}
		}

		if prec == 9 {
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

