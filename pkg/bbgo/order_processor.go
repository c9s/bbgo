package bbgo

import (
	"context"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

var (
	ErrQuoteBalanceLevelTooLow  = errors.New("quote balance level is too low")
	ErrInsufficientQuoteBalance = errors.New("insufficient quote balance")

	ErrAssetBalanceLevelTooLow  = errors.New("asset balance level too low")
	ErrInsufficientAssetBalance = errors.New("insufficient asset balance")
	ErrAssetBalanceLevelTooHigh = errors.New("asset balance level too high")
)

// OrderProcessor does:
//	- Check quote balance
//  - Check and control the order amount
//  - Adjust order amount due to the minAmount configuration and maxAmount configuration
//  - Canonicalize the volume precision base on the given exchange
type OrderProcessor struct {
	// balance control
	MinQuoteBalance float64 `json:"minQuoteBalance"`
	MaxAssetBalance float64 `json:"maxBaseAssetBalance"`
	MinAssetBalance float64 `json:"minBaseAssetBalance"`

	// MinProfitSpread is used when submitting sell orders, it check if there the selling can make the profit.
	MinProfitSpread float64 `json:"minProfitSpread"`

	MaxOrderAmount float64 `json:"maxOrderAmount"`

	Exchange types.Exchange `json:"-"`
	Trader   *Trader        `json:"-"`
}

func (p *OrderProcessor) Submit(ctx context.Context, order *types.SubmitOrder) error {
	/*
	tradingCtx := p.Trader.Context
	currentPrice := tradingCtx.CurrentPrice
	market := order.Market
	quantity := order.Quantity

	tradingCtx.Lock()
	defer tradingCtx.Unlock()

	switch order.Side {
	case types.SideTypeBuy:

		if balance, ok := tradingCtx.Balances[market.QuoteCurrency]; ok {
			if balance.Available < p.MinQuoteBalance {
				return errors.Wrapf(ErrQuoteBalanceLevelTooLow, "quote balance level is too low: %s < %s",
					types.USD.FormatMoneyFloat64(balance.Available),
					types.USD.FormatMoneyFloat64(p.MinQuoteBalance))
			}

			if baseBalance, ok := tradingCtx.Balances[market.BaseCurrency]; ok {
				if util.NotZero(p.MaxAssetBalance) && baseBalance.Available > p.MaxAssetBalance {
					return errors.Wrapf(ErrAssetBalanceLevelTooHigh, "asset balance level is too high: %f > %f", baseBalance.Available, p.MaxAssetBalance)
				}
			}

			available := math.Max(0.0, balance.Available-p.MinQuoteBalance)

			if available < market.MinAmount {
				return errors.Wrapf(ErrInsufficientQuoteBalance, "insufficient quote balance: %f < min amount %f", available, market.MinAmount)
			}

			quantity = adjustQuantityByMinAmount(quantity, currentPrice, market.MinAmount*1.01)
			quantity = adjustQuantityByMaxAmount(quantity, currentPrice, available)
			amount := quantity * currentPrice
			if amount < market.MinAmount {
				return fmt.Errorf("amount too small: %f < min amount %f", amount, market.MinAmount)
			}
		}

	case types.SideTypeSell:

		if balance, ok := tradingCtx.Balances[market.BaseCurrency]; ok {
			if util.NotZero(p.MinAssetBalance) && balance.Available < p.MinAssetBalance {
				return errors.Wrapf(ErrAssetBalanceLevelTooLow, "asset balance level is too low: %f > %f", balance.Available, p.MinAssetBalance)
			}

			quantity = adjustQuantityByMinAmount(quantity, currentPrice, market.MinNotional*1.01)

			available := balance.Available
			quantity = math.Min(quantity, available)
			if quantity < market.MinQuantity {
				return errors.Wrapf(ErrInsufficientAssetBalance, "insufficient asset balance: %f > minimal quantity %f", available, market.MinQuantity)
			}

			notional := quantity * currentPrice
			if notional < tradingCtx.Market.MinNotional {
				return fmt.Errorf("notional %f < min notional: %f", notional, market.MinNotional)
			}

			// price tick10
			// 2 -> 0.01 -> 0.1
			// 4 -> 0.0001 -> 0.001
			tick10 := math.Pow10(-market.PricePrecision + 1)
			minProfitSpread := math.Max(p.MinProfitSpread, tick10)
			estimatedFee := currentPrice * 0.0015 * 2 // double the fee
			targetPrice := currentPrice - estimatedFee - minProfitSpread

			stockQuantity := tradingCtx.StockManager.Stocks.QuantityBelowPrice(targetPrice)
			if math.Round(stockQuantity*1e8) == 0.0 {
				return fmt.Errorf("profitable stock not found: target price %f, profit spread: %f", targetPrice, minProfitSpread)
			}

			quantity = math.Min(quantity, stockQuantity)
			if quantity < market.MinLot {
				return fmt.Errorf("quantity %f less than min lot %f", quantity, market.MinLot)
			}

			notional = quantity * currentPrice
			if notional < tradingCtx.Market.MinNotional {
				return fmt.Errorf("notional %f < min notional: %f", notional, market.MinNotional)
			}
		}
	}

	order.Quantity = quantity
	order.QuantityString = market.FormatVolume(quantity)
	 */

	return p.Exchange.SubmitOrder(ctx, order)
}

func adjustQuantityByMinAmount(quantity float64, currentPrice float64, minAmount float64) float64 {
	// modify quantity for the min amount
	amount := currentPrice * quantity
	if amount < minAmount {
		ratio := minAmount / amount
		quantity *= ratio
	}

	return quantity
}

func adjustQuantityByMaxAmount(quantity float64, currentPrice float64, maxAmount float64) float64 {
	amount := currentPrice * quantity
	if amount > maxAmount {
		ratio := maxAmount / amount
		quantity *= ratio
	}

	return quantity
}
