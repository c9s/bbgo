package accounting

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Stock types.Trade

func (stock *Stock) String() string {
	return stock.Price.String() + " (" + stock.Quantity.String() + ")"
}

func (stock *Stock) Consume(quantity fixedpoint.Value) fixedpoint.Value {
	q := fixedpoint.Min(stock.Quantity, quantity)
	stock.Quantity = stock.Quantity.Sub(q)
	return q
}

type StockSlice []Stock

func (slice StockSlice) QuantityBelowPrice(price fixedpoint.Value) (quantity fixedpoint.Value) {
	for _, stock := range slice {
		if stock.Price.Compare(price) < 0 {
			quantity = quantity.Add(stock.Quantity)
		}
	}

	return quantity
}

func (slice StockSlice) Quantity() (total fixedpoint.Value) {
	for _, stock := range slice {
		total = total.Add(stock.Quantity)
	}

	return total
}

type StockDistribution struct {
	mu sync.Mutex

	Symbol             string
	TradingFeeCurrency string
	Stocks             StockSlice
	PendingSells       StockSlice
}

type DistributionStats struct {
	PriceLevels   []string                    `json:"priceLevels"`
	TotalQuantity fixedpoint.Value            `json:"totalQuantity"`
	Quantities    map[string]fixedpoint.Value `json:"quantities"`
	Stocks        map[string]StockSlice       `json:"stocks"`
}

func (m *StockDistribution) DistributionStats(level int) *DistributionStats {
	var d = DistributionStats{
		Quantities: map[string]fixedpoint.Value{},
		Stocks:     map[string]StockSlice{},
	}

	for _, stock := range m.Stocks {
		n := math.Ceil(math.Log10(stock.Price.Float64()))
		digits := int(n - math.Max(float64(level), 1.0))
		key := stock.Price.Round(-digits, fixedpoint.Down).FormatString(2)

		d.TotalQuantity = d.TotalQuantity.Add(stock.Quantity)
		d.Stocks[key] = append(d.Stocks[key], stock)
		d.Quantities[key] = d.Quantities[key].Add(stock.Quantity)
	}

	var priceLevels []float64
	for priceString := range d.Stocks {
		price, _ := strconv.ParseFloat(priceString, 32)
		priceLevels = append(priceLevels, price)
	}
	sort.Float64s(priceLevels)

	for _, price := range priceLevels {
		d.PriceLevels = append(d.PriceLevels, strconv.FormatFloat(price, 'f', 2, 64))
	}

	return &d
}

func (m *StockDistribution) stock(stock Stock) error {
	m.mu.Lock()
	m.Stocks = append(m.Stocks, stock)
	m.mu.Unlock()
	return m.flushPendingSells()
}

func (m *StockDistribution) squash() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var squashed StockSlice
	for _, stock := range m.Stocks {
		if !stock.Quantity.IsZero() {
			squashed = append(squashed, stock)
		}
	}
	m.Stocks = squashed
}

func (m *StockDistribution) flushPendingSells() error {
	if len(m.Stocks) == 0 || len(m.PendingSells) == 0 {
		return nil
	}

	pendingSells := m.PendingSells
	m.PendingSells = nil

	for _, sell := range pendingSells {
		if err := m.consume(sell); err != nil {
			return err
		}
	}

	return nil
}

func (m *StockDistribution) consume(sell Stock) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Stocks) == 0 {
		m.PendingSells = append(m.PendingSells, sell)
		return nil
	}

	idx := len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]

		// find any stock price is lower than the sell trade
		if stock.Price.Compare(sell.Price) >= 0 {
			continue
		}

		if stock.Quantity.IsZero() {
			continue
		}

		delta := stock.Consume(sell.Quantity)
		sell.Consume(delta)
		m.Stocks[idx] = stock

		if sell.Quantity.IsZero() {
			return nil
		}
	}

	idx = len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]

		if stock.Quantity.IsZero() {
			continue
		}

		delta := stock.Consume(sell.Quantity)
		sell.Consume(delta)
		m.Stocks[idx] = stock

		if sell.Quantity.IsZero() {
			return nil
		}
	}

	if sell.Quantity.Sign() > 0 {
		m.PendingSells = append(m.PendingSells, sell)
	}

	return nil
}

func (m *StockDistribution) AddTrades(trades []types.Trade) (checkpoints []int, err error) {
	feeSymbol := strings.HasPrefix(m.Symbol, m.TradingFeeCurrency)
	for idx, trade := range trades {
		// for other market trades
		// convert trading fee trades to sell trade
		if trade.Symbol != m.Symbol {
			if feeSymbol && trade.FeeCurrency == m.TradingFeeCurrency {
				trade.Symbol = m.Symbol
				trade.IsBuyer = false
				trade.Quantity = trade.Fee
				trade.Fee = fixedpoint.Zero
			}
		}

		if trade.Symbol != m.Symbol {
			continue
		}

		if trade.IsBuyer {
			if idx > 0 && len(m.Stocks) == 0 {
				checkpoints = append(checkpoints, idx)
			}

			stock := toStock(trade)
			if err := m.stock(stock); err != nil {
				return checkpoints, err
			}
		} else {
			stock := toStock(trade)
			if err := m.consume(stock); err != nil {
				return checkpoints, err
			}
		}
	}

	err = m.flushPendingSells()

	m.squash()

	return checkpoints, err
}

func toStock(trade types.Trade) Stock {
	if strings.HasPrefix(trade.Symbol, trade.FeeCurrency) {
		if trade.IsBuyer {
			trade.Quantity = trade.Quantity.Sub(trade.Fee)
		} else {
			trade.Quantity = trade.Quantity.Add(trade.Fee)
		}
		trade.Fee = fixedpoint.Zero
	}
	return Stock(trade)
}
