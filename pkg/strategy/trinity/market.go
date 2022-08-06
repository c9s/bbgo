package trinity

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/types"
)


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
	return m.Symbol
}

func (m *ArbMarket) getInitialBalance(balances types.BalanceMap, dir int) (fixedpoint.Value, string) {
	if dir == 1 { // sell 1 BTC -> 19000 USDT
		b, ok := balances[m.BaseCurrency]
		if !ok {
			return fixedpoint.Zero, m.BaseCurrency
		}

		return m.market.TruncateQuantity(b.Available), m.BaseCurrency
	} else if dir == -1 {
		b, ok := balances[m.QuoteCurrency]
		if !ok {
			return fixedpoint.Zero, m.QuoteCurrency
		}

		return m.market.TruncateQuantity(b.Available), m.QuoteCurrency
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
		q, r := fitQuantityByBase(m.market.TruncateQuantity(m.bestBid.Volume).Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		// fq = m.market.TruncateQuantity(fq)
		return types.SubmitOrder{
			Symbol:   m.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeLimit,
			Quantity: fq,
			Price:    m.bestBid.Price,
			Market:   m.market,
		}, r
	} else if dir == -1 { // use 1 BTC to buy X ETH
		q, r := fitQuantityByQuote(m.bestAsk.Price.Float64(), m.market.TruncateQuantity(m.bestAsk.Volume).Float64(), transitingQuantity)
		fq := fixedpoint.NewFromFloat(q)
		// fq = m.market.TruncateQuantity(fq)
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
