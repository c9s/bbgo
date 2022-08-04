package trinity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "trinity"

var log = logrus.WithField("strategy", ID)

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
}

func (m *ArbMarket) String() string {
	return fmt.Sprintf("%s (%f / %f)", m.Symbol, m.buyRate, m.sellRate)
}

func (m *ArbMarket) updateRate() {
	m.buyRate = 1.0 / m.bestAsk.Price.Float64()
	m.sellRate = m.bestBid.Price.Float64()
}

func buildArbMarkets(session *bbgo.ExchangeSession, symbols []string, separateStream bool) (map[string]*ArbMarket, error) {
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
	var rate = 1.0

	if p.dirA == 1 {
		rate *= p.marketA.buyRate
	} else if p.dirA == -1 {
		rate *= p.marketA.sellRate
	}

	if p.dirC == 1 {
		rate *= p.marketC.buyRate
	} else if p.dirC == -1 {
		rate *= p.marketC.sellRate
	}

	if p.dirB == 1 {
		rate *= p.marketB.buyRate
	} else if p.dirB == -1 {
		rate *= p.marketB.sellRate
	}

	return rate
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

	if p.dirA == 1 { // direct 1 = sell
		ratio *= p.marketA.sellRate
	} else if p.dirA == -1 {
		ratio *= p.marketA.buyRate
	}

	if p.dirB == 1 { // direct 1 = sell
		ratio *= p.marketB.sellRate
	} else if p.dirB == -1 {
		ratio *= p.marketB.buyRate
	}

	if p.dirC == 1 { // direct 1 = sell
		ratio *= p.marketC.sellRate
	} else if p.dirC == -1 {
		ratio *= p.marketC.buyRate
	}

	return ratio
}

func (p *Path) NewForwardOrders(balances types.BalanceMap) []types.SubmitOrder {
	var quantity float64
	var rate = 1.0
	var transitingQuantity float64
	var transitingCurrency string
	var orders []types.SubmitOrder

	// for example BTCUSDT
	if p.dirA == 1 { // sell 1 BTC -> 19000 USDT
		b, ok := balances[p.marketA.BaseCurrency]
		if !ok {
			return nil
		}

		quantity = math.Min(b.Available.Float64(), p.marketA.bestBid.Volume.Float64())
		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketA.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketA.bestBid.Price,
			Market:   p.marketA.market,
		})

		transitingQuantity = quantity * p.marketA.bestBid.Price.Float64() // -> USDT quantity
		transitingCurrency = p.marketA.QuoteCurrency
	} else if p.dirA == -1 { // buy 1 BTC
		b, ok := balances[p.marketA.QuoteCurrency]
		if !ok {
			return nil
		}

		quantity = p.marketA.bestAsk.Volume.Float64() // BTC quantity

		q, r := fitQuantityByQuote(p.marketA.bestAsk.Price.Float64(), p.marketA.bestAsk.Volume.Float64(), b.Available.Float64())
		rate *= r
		quantity = q

		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketA.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketA.bestAsk.Price,
			Market:   p.marketA.market,
		})

		transitingQuantity = quantity // quote quantity
		transitingCurrency = p.marketA.BaseCurrency
	}

	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)

	// for example ETHBTC or USDT/TWD
	if p.dirB == 1 { // sell ETH -> BTC, sell USDT -> TWD
		q, r := fitQuantityByBase(transitingQuantity, p.marketB.bestBid.Volume.Float64())
		rate *= r
		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketB.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketB.bestBid.Price,
			Market:   p.marketB.market,
		})
		transitingQuantity = q * p.marketB.bestBid.Price.Float64()
		transitingCurrency = p.marketB.QuoteCurrency
	} else if p.dirB == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(p.marketB.bestAsk.Price.Float64(), p.marketB.bestAsk.Volume.Float64(), transitingQuantity)
		quantity = q
		rate *= r
		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketB.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketB.bestAsk.Price,
			Market:   p.marketB.market,
		})
		transitingQuantity = quantity
		transitingCurrency = p.marketB.BaseCurrency
	}

	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)

	// for example ETHBTC or USDT/TWD
	if p.dirC == 1 { // sell ETH -> BTC, sell USDT -> TWD
		q, r := fitQuantityByBase(transitingQuantity, p.marketC.bestBid.Volume.Float64())
		rate *= r
		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketC.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketC.bestBid.Price,
			Market:   p.marketC.market,
		})
		transitingQuantity = q * p.marketC.bestBid.Price.Float64()
		transitingCurrency = p.marketC.QuoteCurrency
	} else if p.dirC == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(p.marketC.bestAsk.Price.Float64(), p.marketC.bestAsk.Volume.Float64(), transitingQuantity)
		quantity = q
		rate *= r
		orders = append(orders, types.SubmitOrder{
			Symbol:   p.marketC.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeLimit,
			Quantity: fixedpoint.NewFromFloat(quantity),
			Price:    p.marketC.bestAsk.Price,
			Market:   p.marketC.market,
		})
		transitingQuantity = quantity
		transitingCurrency = p.marketC.BaseCurrency
	}

	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)
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

type State struct{}

type Strategy struct {
	Symbols        []string         `json:"symbols"`
	Paths          [][]string       `json:"paths"`
	MinSpreadRatio fixedpoint.Value `json:"minSpreadRatio"`
	SeparateStream bool             `json:"separateStream"`

	markets map[string]*ArbMarket
	paths   []*Path

	session *bbgo.ExchangeSession

	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *bbgo.OrderStore
	tradeStore        *bbgo.TradeStore
}

func (s *Strategy) ID() string {
	return ID
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
	if s.MinSpreadRatio.IsZero() {
		s.MinSpreadRatio = fixedpoint.NewFromFloat(1.002)
	}

	s.session = session
	s.orderStore = bbgo.NewOrderStore("")
	s.orderStore.BindStream(session.UserDataStream)

	arbMarkets, err := buildArbMarkets(session, s.Symbols, s.SeparateStream)
	if err != nil {
		return err
	}

	s.markets = arbMarkets

	for _, market := range s.markets {
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
			marketA: s.markets[symbols[0]],
			marketB: s.markets[symbols[1]],
			marketC: s.markets[symbols[2]],
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
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()

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
			case <-ticker.C:
				minRatio := s.MinSpreadRatio.Float64()
				for side, f := range fs {
					ranks := s.calculateRanks(minRatio, f)
					if len(ranks) == 0 {
						break
					}
					s.executePath(session, ranks[0].Path, side == 0)
				}
			}
		}
	}()

	return nil
}

type ratioFunction func(p *Path) float64

func (s *Strategy) executePath(session *bbgo.ExchangeSession, p *Path, dir bool) {
	log.Infof("executing path: %+v", p)
	balances := session.Account.Balances()
	if dir {
		orders := p.NewForwardOrders(balances)
		_ = orders
	} else {

	}
}

func (s *Strategy) calculateRanks(minRatio float64, method func(p *Path) float64) []PathRank {
	ranks := make([]PathRank, 0, len(s.paths))

	// ranking paths here
	for i, path := range s.paths {
		_ = i
		ratio := method(path)
		log.Infof("path #%d: ratio:%f path: %+v", i, ratio, path)
		if ratio >= minRatio {
			p := path
			ranks = append(ranks, PathRank{Path: p, Ratio: ratio})
		}
	}

	// sort and pick up the top rank path
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].Ratio > ranks[j].Ratio
	})

	return ranks
}

func fitQuantityByBase(quantity, balance float64) (float64, float64) {
	rate := 1.0

	if quantity > balance {
		quantity = math.Min(quantity, balance)
		rate = balance / quantity
	}

	return quantity, rate
}

// 1620 x 2 , quote balance = 1000 => rate = 1000/(1620*2) = 0.3086419753, quantity = 0.61728395
func fitQuantityByQuote(price, quantity, quoteBalance float64) (float64, float64) {
	rate := 1.0
	quote := quantity * price
	if quote > quoteBalance {
		// rate = quoteBalance / quote
		// quantity = quantity * rate
		// or we can calculate it by:
		//   quantity = quoteBalance / price
		quantity = quoteBalance / price
		rate = quoteBalance / quote
	}

	return quantity, rate
}
