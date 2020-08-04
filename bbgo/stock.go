package bbgo

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	log "github.com/sirupsen/logrus"
	"math"
	"strings"
)

type Stock types.Trade

func (stock *Stock) Consume(quantity float64) float64 {
	delta := math.Min(stock.Quantity, quantity)
	stock.Quantity -= delta
	return delta
}

type StockManager struct {
	Symbol string
	TradingFeeCurrency string
	Stocks []Stock
}

func (m *StockManager) Consume(sell Stock) error {
	if len(m.Stocks) == 0 {
		return fmt.Errorf("empty stock")
	}

	idx := len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]

		// find any stock price is lower than the sell trade
		if stock.Price >= sell.Price {
			continue
		}

		sell.Quantity -= stock.Consume(sell.Quantity)

		if math.Round(stock.Quantity*1e8) < 0.0 {
			return fmt.Errorf("over sell")
		}

		if math.Round(stock.Quantity*1e8) == 0.0 {
			m.Stocks = m.Stocks[:idx]
		}

		if math.Round(sell.Quantity*1e8) == 0.0 {
			break
		}
	}

	idx = len(m.Stocks) - 1
	for ; idx >= 0; idx-- {
		stock := m.Stocks[idx]

		sell.Quantity -= stock.Consume(sell.Quantity)
		if math.Round(stock.Quantity*1e8) == 0.0 {
			m.Stocks = m.Stocks[:idx]
		}

		if math.Round(sell.Quantity*1e8) == 0.0 {
			break
		}
	}

	if math.Round(sell.Quantity*1e8) > 0.0 {
		return fmt.Errorf("over sell quantity %f at %s", sell.Quantity, sell.Time)
	}


	return nil
}

func (m *StockManager) LoadTrades(trades []types.Trade) (checkpoints []int, err error) {
	feeSymbol := strings.HasPrefix(m.Symbol, m.TradingFeeCurrency)
	for idx, trade := range trades {
		log.Infof("%s %5s %f %f at %s", trade.Symbol, trade.Side, trade.Price, trade.Quantity, trade.Time)
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
				m.Stocks = append(m.Stocks, toStock(trade))
			} else {
				if err := m.Consume(toStock(trade)) ; err != nil {
					return checkpoints, err
				}
			}

			if len(m.Stocks) == 0 {
				checkpoints = append(checkpoints, idx)
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
