package coinbase

import (
	"context"
	"math"
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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

func toGlobalOrderStatus(cbStatus api.OrderStatus, doneReason string) types.OrderStatus {
	switch cbStatus {
	case api.OrderStatusRejected:
		return types.OrderStatusRejected
	case api.OrderStatusReceived, api.OrderStatusOpen, api.OrderStatusPending:
		return types.OrderStatusNew
	case api.OrderStatusDone:
		switch doneReason {
		case "filled":
			return types.OrderStatusFilled
		case "canceled":
			return types.OrderStatusCanceled
		case "rejected":
			return types.OrderStatusRejected
		}
	case api.OrderStatusCanceled:
		return types.OrderStatusCanceled
	case api.OrderStatusFilled:
		return types.OrderStatusFilled
	}
	return types.OrderStatus(strings.ToUpper(string(cbStatus)))
}

func toGlobalOrder(cbOrder *api.Order) types.Order {
	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: cbOrder.ClientOID,
			Symbol:        toGlobalSymbol(cbOrder.ProductID),
			Type:          toGlobalOrderType(cbOrder.Type),
			Side:          toGlobalSide(cbOrder.Side),
			Quantity:      cbOrder.Size,
			Price:         cbOrder.Price,
			StopPrice:     cbOrder.StopPrice,
			TimeInForce:   toGlobalTimeInForce(cbOrder.TimeInForce),
		},
		Exchange:         types.ExchangeCoinBase,
		Status:           toGlobalOrderStatus(cbOrder.Status, cbOrder.DoneReason),
		UUID:             cbOrder.ID,
		OrderID:          util.FNV64(cbOrder.ID),
		OriginalStatus:   string(cbOrder.Status),
		CreationTime:     cbOrder.CreatedAt,
		IsWorking:        isWorkingOrder(cbOrder.Status),
		ExecutedQuantity: cbOrder.FilledSize,
	}
	if cbOrder.Status == api.OrderStatusDone {
		order.UpdateTime = cbOrder.DoneAt
	} else {
		order.UpdateTime = cbOrder.CreatedAt
	}
	return order
}

func submitOrderToGlobalOrder(submitOrder types.SubmitOrder, res *api.CreateOrderResponse) *types.Order {
	return &types.Order{
		SubmitOrder:      submitOrder,
		Exchange:         types.ExchangeCoinBase,
		OrderID:          util.FNV64(res.ID),
		UUID:             res.ID,
		Status:           toGlobalOrderStatus(res.Status, res.DoneReason),
		ExecutedQuantity: res.FilledSize,
		IsWorking:        !res.FilledSize.Eq(submitOrder.Quantity),
		CreationTime:     res.CreatedAt,
		UpdateTime:       res.CreatedAt,
		OriginalStatus:   string(res.Status),
	}
}

func toGlobalTrade(cbTrade *api.Trade) types.Trade {
	side := toGlobalSide(cbTrade.Side)
	return types.Trade{
		ID:            cbTrade.TradeID,
		OrderID:       util.FNV64(cbTrade.OrderID),
		OrderUUID:     cbTrade.OrderID,
		Exchange:      types.ExchangeCoinBase,
		Price:         cbTrade.Price,
		Quantity:      cbTrade.Size,
		QuoteQuantity: cbTrade.Size.Mul(cbTrade.Price),
		Symbol:        toGlobalSymbol(cbTrade.ProductID),
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       cbTrade.Liquidity == api.LiquidityMaker,
		Fee:           cbTrade.Fee,
		FeeCurrency:   cbTrade.FundingCurrency,
		FeeProcessing: !cbTrade.Settled,
		Time:          cbTrade.CreatedAt,
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

// stream message convert functions

// Trade() convert the message to a Trade
// fee currency on Coinbase is USD:
// https://help.coinbase.com/en/exchange/trading-and-funding/exchange-fees
func (msg *MatchMessage) Trade(s *Stream) types.Trade {
	var side types.SideType
	// NOTE: the message side is the maker side
	switch msg.Side {
	case "buy":
		side = types.SideTypeBuy
	case "sell":
		side = types.SideTypeSell
	default:
		side = types.SideType(msg.Side)
	}
	quoteQuantity := msg.Size.Mul(msg.Price)
	orderUUID := ""
	if msg.UserID != "" {
		// it's an match of authenticated user, which means it's from a user data stream
		switch msg.UserID {
		case msg.TakerUserID:
			// the user is the taker
			orderUUID = msg.TakerOrderID
			side = side.Reverse() // the user is on the reverse side of the maker side
		case msg.MakerUserID:
			// the user is the maker
			orderUUID = msg.MakerOrderID
		}
	}
	var orderID uint64 = 0
	if orderUUID != "" {
		orderID = util.FNV64(orderUUID)
	}
	var quoteCurrency string
	market, ok := s.marketInfoMap[msg.ProductID]
	if ok {
		quoteCurrency = market.QuoteCurrency
	} else {
		logger.Warnf("unknown product id: %s", msg.ProductID)
		quoteCurrency = "USD" // fallback to USD
	}
	return types.Trade{
		ID:            uint64(msg.TradeID),
		Exchange:      types.ExchangeCoinBase,
		OrderID:       orderID,
		OrderUUID:     orderUUID,
		Price:         msg.Price,
		Quantity:      msg.Size,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		Symbol:        toGlobalSymbol(msg.ProductID),
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       msg.IsAuthMaker(),
		Time:          types.Time(msg.Time),
		FeeProcessing: true, // assume the fee is processing when a match happens
		FeeCurrency:   quoteCurrency,
		Fee:           quoteQuantity.Mul(msg.FeeRate()),
	}
}

func (m *ReceivedMessage) Order(s *Stream) types.Order {
	var order *types.Order
	if activeOrder, ok := s.exchange.activeOrderStore.get(m.OrderID); ok {
		order = submitOrderToGlobalOrder(activeOrder.submitOrder, activeOrder.rawOrder)
	} else {
		// query the order if not found in active orders
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		createdOrder, err := s.exchange.QueryOrder(ctx, types.OrderQuery{OrderUUID: m.OrderID})
		if err == nil {
			order = createdOrder
		} else {
			logger.Warnf("fail to retrieve order info for received message: %s", m.OrderID)
			order = &types.Order{
				SubmitOrder: types.SubmitOrder{
					Symbol: toGlobalSymbol(m.ProductID),
					Side:   toGlobalSide(m.Side),
				},
				OrderID: util.FNV64(m.OrderID),
				UUID:    m.OrderID,
			}
		}
	}
	order.Exchange = types.ExchangeCoinBase
	order.Status = types.OrderStatusNew
	order.UpdateTime = types.Time(m.Time)
	order.IsWorking = true

	switch m.OrderType {
	case "limit":
		order.SubmitOrder.Type = types.OrderTypeLimit
		order.SubmitOrder.Price = m.Price
		order.SubmitOrder.Quantity = m.Size
	case "market":
		// NOTE: the Exchange.SubmitOrder method guarantees that the market order does not support funds.
		// So we simply use the size for market order here.
		order.SubmitOrder.Type = types.OrderTypeMarket
		order.SubmitOrder.Quantity = m.Size
	}
	return *order
}
