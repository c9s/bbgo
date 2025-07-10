package coinbase

import (
	"hash/fnv"
	"math"
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSide(cbSide api.SideType) types.SideType {
	switch cbSide {
	case api.SideTypeBuy:
		return types.SideTypeBuy
	case api.SideTypeSell:
		return types.SideTypeSell
	}
	return types.SideTypeNone
}

func toGlobalOrderStatus(order *api.Order) types.OrderStatus {
	switch order.Status {
	case api.OrderStatusRejected:
		return types.OrderStatusRejected
	case api.OrderStatusReceived, api.OrderStatusOpen, api.OrderStatusPending:
		return types.OrderStatusNew
	case api.OrderStatusDone:
		switch order.DoneReason {
		case "filled":
			return types.OrderStatusFilled
		case "canceled":
			return types.OrderStatusCanceled
		case "rejected":
			return types.OrderStatusRejected
		}
	}
	return types.OrderStatus(strings.ToUpper(string(order.Status)))
}

func toGlobalOrder(cbOrder *api.Order) types.Order {
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: cbOrder.ClientOID,
			Type:          toGlobalOrderType(cbOrder.Type),
			Side:          toGlobalSide(cbOrder.Side),
			Quantity:      cbOrder.Size,
			Price:         cbOrder.Price,
			StopPrice:     cbOrder.StopPrice,
			TimeInForce:   toGlobalTimeInForce(cbOrder.TimeInForce),
		},
		Exchange:       types.ExchangeCoinBase,
		Status:         toGlobalOrderStatus(cbOrder),
		UUID:           cbOrder.ID,
		OrderID:        FNV64a(cbOrder.ID),
		OriginalStatus: string(cbOrder.Status),
		CreationTime:   cbOrder.CreatedAt,
		IsWorking:      isWorkingOrder(cbOrder.Status),
	}
}

func toGlobalTrade(cbTrade *api.Trade) types.Trade {
	return types.Trade{
		ID:            cbTrade.TradeID,
		OrderID:       FNV64a(cbTrade.OrderID),
		Exchange:      types.ExchangeCoinBase,
		Price:         cbTrade.Price,
		Quantity:      cbTrade.Size,
		QuoteQuantity: cbTrade.Size.Mul(cbTrade.Price),
		Symbol:        cbTrade.ProductID,
		Side:          toGlobalSide(cbTrade.Side),
		IsBuyer:       cbTrade.Liquidity == api.LiquidityTaker,
		IsMaker:       cbTrade.Liquidity == api.LiquidityMaker,
		Fee:           cbTrade.Fee,
		FeeCurrency:   cbTrade.FundingCurrency,
	}
}

// The max order size is estimated according to the trading rules. See below:
// - https://www.coinbase.com/legal/trading_rules
//
// According to the markets list, the PPP is the max slippage percentage:
// - https://exchange.coinbase.com/markets
func toGlobalMarket(cbMarket *api.MarketInfo) types.Market {
	pricePrecision := int(math.Log10(fixedpoint.One.Div(cbMarket.QuoteIncrement).Float64()))
	volumnPrecision := int(math.Log10(fixedpoint.One.Div(cbMarket.BaseIncrement).Float64()))

	// NOTE: Coinbase does not appose a min quantity, but a min notional.
	// So we set the min quantity to the base increment. Or it may require more API calls
	// to calculate the exact min quantity, which is costy.
	minQuantity := cbMarket.BaseIncrement
	// TODO: estimate max quantity by PPP
	// fill a dummy value for now.
	maxQuantity := minQuantity.Mul(fixedpoint.NewFromFloat(1.5))

	return types.Market{
		Exchange:        types.ExchangeCoinBase,
		Symbol:          toGlobalSymbol(cbMarket.ID),
		LocalSymbol:     cbMarket.ID,
		PricePrecision:  pricePrecision,
		VolumePrecision: volumnPrecision,
		QuoteCurrency:   cbMarket.QuoteCurrency,
		BaseCurrency:    cbMarket.BaseCurrency,
		MinNotional:     cbMarket.MinMarketFunds,
		MinAmount:       cbMarket.MinMarketFunds,
		TickSize:        cbMarket.QuoteIncrement,
		StepSize:        cbMarket.BaseIncrement,
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
		MinQuantity:     minQuantity,
		MaxQuantity:     maxQuantity,
	}
}

func FNV64a(text string) uint64 {
	hash := fnv.New64a()
	// In hash implementation, it says never return an error.
	_, _ = hash.Write([]byte(text))
	return hash.Sum64()
}

func toGlobalKline(symbol string, interval types.Interval, candle *api.Candle) types.KLine {
	startTime := candle.Time.Time()
	endTime := startTime.Add(interval.Duration())
	kline := types.KLine{
		Exchange:  types.ExchangeCoinBase,
		Symbol:    symbol,
		StartTime: types.Time(startTime),
		EndTime:   types.Time(endTime),
		Interval:  interval,
		Open:      candle.Open,
		Close:     candle.Close,
		High:      candle.High,
		Low:       candle.Low,
		Volume:    candle.Volume,
	}
	return kline
}

func toGlobalTicker(cbTicker *api.Ticker) types.Ticker {
	ticker := types.Ticker{
		Time:   time.Time(cbTicker.Time),
		Volume: cbTicker.Volume,
		Buy:    cbTicker.Bid,
		Sell:   cbTicker.Ask,
	}
	return ticker
}

func toGlobalBalance(cur string, cbBalance *api.Balance) types.Balance {
	balance := types.NewZeroBalance(cur)
	balance.Currency = cur
	balance.Available = cbBalance.Available
	balance.Locked = cbBalance.Hold
	balance.NetAsset = cbBalance.Balance
	return balance
}

func toGlobalOrderType(localType string) types.OrderType {
	switch localType {
	case "limit":
		return types.OrderTypeLimit
	case "market":
		return types.OrderTypeMarket
	case "stop":
		return types.OrderTypeStopLimit
	default:
		return types.OrderType(strings.ToUpper(localType))
	}
}

func toGlobalTimeInForce(localTIF api.TimeInForceType) types.TimeInForce {
	switch localTIF {
	case api.TimeInForceGTC:
		return types.TimeInForceGTC
	case api.TimeInForceIOC:
		return types.TimeInForceIOC
	case api.TimeInForceFOK:
		return types.TimeInForceFOK
	case api.TimeInForceGTT:
		return types.TimeInForceGTT
	default:
		return types.TimeInForce(strings.ToUpper(string(localTIF)))
	}
}

func toGlobalDeposit(transfer *api.Transfer) types.Deposit {
	createTime := transfer.CreatedAt.Time()
	cancelTime := transfer.CanceledAt.Time()
	completeTime := transfer.CompletedAt.Time()
	deposit := types.Deposit{
		Exchange:      types.ExchangeCoinBase,
		Time:          types.Time(createTime),
		Amount:        transfer.Amount,
		Asset:         transfer.Currency,
		Address:       transfer.Details.CryptoAddress,
		TransactionID: transfer.ID,
		Network:       transfer.Details.Network,
	}
	switch {
	case !cancelTime.IsZero():
		// canceled_at is not zero -> canceled
		deposit.Status = types.DepositCancelled
	case !completeTime.IsZero():
		// completed_at is not zero -> completed
		deposit.Status = types.DepositSuccess
	default:
		deposit.Status = types.DepositPending
	}
	return deposit
}

func toGlobalWithdraw(transfer *api.Transfer) types.Withdraw {
	createTime := transfer.CreatedAt.Time()
	cancelTime := transfer.CanceledAt.Time()
	completeTime := transfer.CompletedAt.Time()
	withdraw := types.Withdraw{
		Exchange: types.ExchangeCoinBase,
		Asset:    transfer.Currency,
		Amount:   transfer.Amount,
		Address:  transfer.Details.SendToAddress,

		TransactionID:          transfer.ID,
		TransactionFee:         transfer.Details.Fee,
		TransactionFeeCurrency: transfer.Currency,
		ApplyTime:              types.Time(createTime),
		Network:                transfer.Details.Network,
	}
	switch {
	case !cancelTime.IsZero():
		// canceled_at is not zero -> canceled
		withdraw.Status = types.WithdrawStatusCancelled
	case !completeTime.IsZero():
		// completed_at is not zero -> completed
		withdraw.Status = types.WithdrawStatusCompleted
	default:
		withdraw.Status = types.WithdrawStatusProcessing
	}
	return withdraw
}

func isWorkingOrder(status api.OrderStatus) bool {
	switch status {
	case api.OrderStatusRejected, api.OrderStatusDone:
		return false
	case api.OrderStatusReceived, api.OrderStatusOpen, api.OrderStatusPending:
		return true
	default:
		return false
	}
}
