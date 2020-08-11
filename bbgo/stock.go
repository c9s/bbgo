package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"math"
	"sort"
	"strings"
	"sync"
)

func zero(a float64) bool {
	return int(math.Round(a*1e8)) == 0
}

func round(a float64) float64 {
	return math.Round(a*1e8) / 1e8
}

type Stock types.Trade

func (stock *Stock) String() string {
	return fmt.Sprintf("%f (%f)", stock.Price, stock.Quantity)
}

func (stock *Stock) Consume(quantity float64) float64 {
	q := math.Min(stock.Quantity, quantity)
	stock.Quantity = round(stock.Quantity - q)
	return q
}

type StockSlice []Stock

func (slice StockSlice) QuantityBelowPrice(price float64) (quantity float64) {
	for _, stock := range slice {
		if stock.Price < price {
			quantity += stock.Quantity
		}
	}

	return round(quantity)
}

func (slice StockSlice) Quantity() (total float64) {
	for _, stock := range slice {
		total += stock.Quantity
	}

	return round(total)
}

type StockManager struct {
	mu sync.Mutex

	Symbol             string
	TradingFeeCurrency string
	Stocks             StockSlice
	PendingSells       StockSlice
}

type Distribution struct {
	PriceLevels []int `json:"priceLevels"`
	TotalQuantity float64 `json:"totalQuantity"`
	Quantities map[int]float64 `json:"quantities"`
	Stocks map[int]StockSlice `json:"stocks"`
}

func (m *StockManager) Distribution(level int) *Distribution {
	var d = Distribution{
		Quantities: map[int]float64{},
		Stocks: map[int]StockSlice{},
	}

	for _, stock := range m.Stocks {
		n := math.Ceil(math.Log10(stock.Price))
		digits := int(n - math.Max(float64(level), 1.0))
		div := math.Pow10(digits)
		priceLevel := int(math.Floor(stock.Price / div) * div)

		d.TotalQuantity += stock.Quantity
		d.Stocks[priceLevel] = append(d.Stocks[priceLevel], stock)
		d.Quantities[priceLevel] += stock.Quantity
	}

	for level := range d.Stocks {
		d.PriceLevels = append(d.PriceLevels, level)
	}

	sort.Ints(d.PriceLevels)

	return &d
}

func (m *StockManager) stock(stock Stock) error {
	m.mu.Lock()
	m.Stocks = append(m.Stocks, stock)
	m.mu.Unlock()
	return m.flushPendingSells()
}

func (m *StockManager) squash() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var squashed StockSlice
	for _, stock := range m.Stocks {
		if !zero(stock.Quantity) {
			squashed = append(squashed, stock)
		}
	}
	m.Stocks = squashed
}

func (m *StockManager) flushPendingSells() error {
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

func (m *StockManager) consume(sell Stock) error {
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
		if stock.Price >= sell.Price {
			continue
		}

		if zero(stock.Quantity) {
			continue
		}

		delta := stock.Consume(sell.Quantity)
		sell.Consume(delta)
		m.Stocks[idx] = stock

		if zero(sell.Quantity) {
			return nil
		}
	}

	idx = len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]

		if zero(stock.Quantity) {
			continue
		}

		delta := stock.Consume(sell.Quantity)
		sell.Consume(delta)
		m.Stocks[idx] = stock

		if zero(sell.Quantity) {
			return nil
		}
	}

	if !zero(sell.Quantity) {
		m.PendingSells = append(m.PendingSells, sell)
	}

	return nil
}

func (m *StockManager) AddTrades(trades []types.Trade) (checkpoints []int, err error) {
	feeSymbol := strings.HasPrefix(m.Symbol, m.TradingFeeCurrency)
	for idx, trade := range trades {
		// for other market trades
		// convert trading fee trades to sell trade
		if trade.Symbol != m.Symbol {
			if feeSymbol && trade.FeeCurrency == m.TradingFeeCurrency {
				trade.Symbol = m.Symbol
				trade.IsBuyer = false
				trade.Quantity = trade.Fee
				trade.Fee = 0.0
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
			trade.Quantity -= trade.Fee
		} else {
			trade.Quantity += trade.Fee
		}
		trade.Fee = 0.0
	}
	return Stock(trade)
}
