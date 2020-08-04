package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	log "github.com/sirupsen/logrus"
	"math"
	"strings"
)

type Stock types.Trade

func (stock *Stock) String() string {
	return fmt.Sprintf("%f (%f)", stock.Price, stock.Quantity)
}

func (stock *Stock) Consume(quantity float64) float64 {
	delta := math.Min(stock.Quantity, quantity)
	stock.Quantity -= delta
	return delta
}

type StockSlice []Stock

func (slice StockSlice) Quantity() (total float64) {
	for _, stock := range slice {
		total += stock.Quantity
	}

	return total
}

type StockManager struct {
	Symbol             string
	TradingFeeCurrency string
	Stocks             StockSlice
	PendingSells       StockSlice
}

func (m *StockManager) Stock(buy Stock) error {
	m.Stocks = append(m.Stocks, buy)

	if len(m.PendingSells) > 0 {
		pendingSells := m.PendingSells
		m.PendingSells = nil
		for _, sell := range pendingSells {
			if err := m.Consume(sell); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *StockManager) Consume(sell Stock) error {
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

		sell.Quantity -= stock.Consume(sell.Quantity)
		m.Stocks[idx] = stock

		if math.Round(stock.Quantity*1e8) == 0.0 {
			m.Stocks = m.Stocks[:idx]
		}

		if math.Round(sell.Quantity*1e8) == 0.0 {
			return nil
		}
	}

	idx = len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]
		sell.Quantity -= stock.Consume(sell.Quantity)
		m.Stocks[idx] = stock

		if math.Round(stock.Quantity*1e8) == 0.0 {
			// remove the latest stock
			m.Stocks = m.Stocks[:idx]
		}
	}

	if math.Round(sell.Quantity*1e8) > 0.0 {
		m.PendingSells = append(m.PendingSells, sell)
	}

	return nil
}

func (m *StockManager) LoadTrades(trades []types.Trade) (checkpoints []int, err error) {
	feeSymbol := strings.HasPrefix(m.Symbol, m.TradingFeeCurrency)
	for idx, trade := range trades {
		log.Infof("%s %5s %f %f at %s fee %s %f", trade.Symbol, trade.Side, trade.Price, trade.Quantity, trade.Time, trade.FeeCurrency, trade.Fee)
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

		if trade.Symbol == m.Symbol {
			if trade.IsBuyer {
				if idx > 0 && len(m.Stocks) == 0 {
					checkpoints = append(checkpoints, idx)
				}

				stock := toStock(trade)
				if err := m.Stock(stock); err != nil {
					return checkpoints, err
				}
			} else {
				stock := toStock(trade)
				if err := m.Consume(stock); err != nil {
					return checkpoints, err
				}
			}
		}
	}

	return checkpoints, nil
}

func toStock(trade types.Trade) Stock {
	if strings.HasPrefix(trade.Symbol, trade.FeeCurrency) {
		if trade.IsBuyer {
			trade.Quantity -= trade.Fee
		} else {
			trade.Quantity += trade.Fee
		}
		trade.Fee = 0
	}
	return Stock(trade)
}
