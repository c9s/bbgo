package bitget

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalBalance(asset bitgetapi.AccountAsset) types.Balance {
	return types.Balance{
		Currency:          asset.CoinName,
		Available:         asset.Available,
		Locked:            asset.Lock.Add(asset.Frozen),
		Borrowed:          fixedpoint.Zero,
		Interest:          fixedpoint.Zero,
		NetAsset:          fixedpoint.Zero,
		MaxWithdrawAmount: fixedpoint.Zero,
	}
}

func toGlobalMarket(s v2.Symbol) types.Market {
	if s.Status != v2.SymbolOnline {
		log.Warnf("The symbol %s is not online", s.Symbol)
	}
	return types.Market{
		Symbol:          s.Symbol,
		LocalSymbol:     s.Symbol,
		PricePrecision:  s.PricePrecision.Int(),
		VolumePrecision: s.QuantityPrecision.Int(),
		QuoteCurrency:   s.QuoteCoin,
		BaseCurrency:    s.BaseCoin,
		MinNotional:     s.MinTradeUSDT,
		MinAmount:       s.MinTradeUSDT,
		MinQuantity:     s.MinTradeAmount,
		MaxQuantity:     s.MaxTradeAmount,
		StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(s.QuantityPrecision.Int())),
		TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(s.PricePrecision.Int())),
		MinPrice:        fixedpoint.Zero,
		MaxPrice:        fixedpoint.Zero,
	}
}

func toGlobalTicker(ticker bitgetapi.Ticker) types.Ticker {
	return types.Ticker{
		Time:   ticker.Ts.Time(),
		Volume: ticker.BaseVol,
		Last:   ticker.Close,
		Open:   ticker.OpenUtc0,
		High:   ticker.High24H,
		Low:    ticker.Low24H,
		Buy:    ticker.BuyOne,
		Sell:   ticker.SellOne,
	}
}

func toGlobalSideType(side v2.SideType) (types.SideType, error) {
	switch side {
	case v2.SideTypeBuy:
		return types.SideTypeBuy, nil

	case v2.SideTypeSell:
		return types.SideTypeSell, nil

	default:
		return types.SideType(side), fmt.Errorf("unexpected side: %s", side)
	}
}

func toGlobalOrderType(s v2.OrderType) (types.OrderType, error) {
	switch s {
	case v2.OrderTypeMarket:
		return types.OrderTypeMarket, nil

	case v2.OrderTypeLimit:
		return types.OrderTypeLimit, nil

	default:
		return types.OrderType(s), fmt.Errorf("unexpected order type: %s", s)
	}
}

func toGlobalOrderStatus(status v2.OrderStatus) (types.OrderStatus, error) {
	switch status {
	case v2.OrderStatusInit, v2.OrderStatusNew, v2.OrderStatusLive:
		return types.OrderStatusNew, nil

	case v2.OrderStatusPartialFilled:
		return types.OrderStatusPartiallyFilled, nil

	case v2.OrderStatusFilled:
		return types.OrderStatusFilled, nil

	case v2.OrderStatusCancelled:
		return types.OrderStatusCanceled, nil

	default:
		return types.OrderStatus(status), fmt.Errorf("unexpected order status: %s", status)
	}
}

func isMaker(s v2.TradeScope) (bool, error) {
	switch s {
	case v2.TradeMaker:
		return true, nil

	case v2.TradeTaker:
		return false, nil

	default:
		return false, fmt.Errorf("unexpected trade scope: %s", s)
	}
}

func isFeeDiscount(s v2.DiscountStatus) (bool, error) {
	switch s {
	case v2.DiscountYes:
		return true, nil

	case v2.DiscountNo:
		return false, nil

	default:
		return false, fmt.Errorf("unexpected discount status: %s", s)
	}
}

func toGlobalTrade(trade v2.Trade) (*types.Trade, error) {
	side, err := toGlobalSideType(trade.Side)
	if err != nil {
		return nil, err
	}

	isMaker, err := isMaker(trade.TradeScope)
	if err != nil {
		return nil, err
	}

	isDiscount, err := isFeeDiscount(trade.FeeDetail.Deduction)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            uint64(trade.TradeId),
		OrderID:       uint64(trade.OrderId),
		Exchange:      types.ExchangeBitget,
		Price:         trade.PriceAvg,
		Quantity:      trade.Size,
		QuoteQuantity: trade.Amount,
		Symbol:        trade.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       isMaker,
		Time:          types.Time(trade.CTime),
		Fee:           trade.FeeDetail.TotalFee.Abs(),
		FeeCurrency:   trade.FeeDetail.FeeCoin,
		FeeDiscounted: isDiscount,
	}, nil
}

// unfilledOrderToGlobalOrder convert the local order to global.
//
// Note that the quantity unit, according official document: Base coin when orderType=limit; Quote coin when orderType=market
// https://bitgetlimited.github.io/apidoc/zh/spot/#19671a1099
func unfilledOrderToGlobalOrder(order v2.UnfilledOrder) (*types.Order, error) {
	side, err := toGlobalSideType(order.Side)
	if err != nil {
		return nil, err
	}

	orderType, err := toGlobalOrderType(order.OrderType)
	if err != nil {
		return nil, err
	}

	status, err := toGlobalOrderStatus(order.Status)
	if err != nil {
		return nil, err
	}

	qty := order.Size
	price := order.PriceAvg

	// The market order will be executed immediately, so this check is used to handle corner cases.
	if orderType == types.OrderTypeMarket {
		qty = order.BaseVolume
		log.Warnf("!!!  The price(%f) and quantity(%f) are not verified for market orders, because we only receive limit orders in the test environment !!!", price.Float64(), qty.Float64())
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: order.ClientOrderId,
			Symbol:        order.Symbol,
			Side:          side,
			Type:          orderType,
			Quantity:      qty,
			Price:         price,
			// Bitget does not include the "time-in-force" field in its API response for spot trading, so we set GTC.
			TimeInForce: types.TimeInForceGTC,
		},
		Exchange:         types.ExchangeBitget,
		OrderID:          uint64(order.OrderId),
		UUID:             strconv.FormatInt(int64(order.OrderId), 10),
		Status:           status,
		ExecutedQuantity: order.BaseVolume,
		IsWorking:        order.Status.IsWorking(),
		CreationTime:     types.Time(order.CTime.Time()),
		UpdateTime:       types.Time(order.UTime.Time()),
	}, nil
}

func toGlobalOrder(order v2.OrderDetail) (*types.Order, error) {
	side, err := toGlobalSideType(order.Side)
	if err != nil {
		return nil, err
	}

	orderType, err := toGlobalOrderType(order.OrderType)
	if err != nil {
		return nil, err
	}

	status, err := toGlobalOrderStatus(order.Status)
	if err != nil {
		return nil, err
	}

	qty := order.Size
	price := order.Price

	if orderType == types.OrderTypeMarket {
		price = order.PriceAvg
		if side == types.SideTypeBuy {
			qty, err = processMarketBuyQuantity(order.BaseVolume, order.QuoteVolume, order.PriceAvg, order.Size, order.Status)
			if err != nil {
				return nil, err
			}
		}
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: order.ClientOrderId,
			Symbol:        order.Symbol,
			Side:          side,
			Type:          orderType,
			Quantity:      qty,
			Price:         price,
			// Bitget does not include the "time-in-force" field in its API response for spot trading, so we set GTC.
			TimeInForce: types.TimeInForceGTC,
		},
		Exchange:         types.ExchangeBitget,
		OrderID:          uint64(order.OrderId),
		UUID:             strconv.FormatInt(int64(order.OrderId), 10),
		Status:           status,
		ExecutedQuantity: order.BaseVolume,
		IsWorking:        order.Status.IsWorking(),
		CreationTime:     types.Time(order.CTime.Time()),
		UpdateTime:       types.Time(order.UTime.Time()),
	}, nil
}

// processMarketBuyQuantity returns the estimated base quantity or real. The order size will be 'quote quantity' when side is buy and
// type is market, so we need to convert that. This is because the unit of types.Order.Quantity is base coin.
//
// If the order status is PartialFilled, return estimated base coin quantity.
// If the order status is Filled, return the filled base quantity instead of the buy quantity, because a market order on the buy side
// cannot execute all.
// Otherwise, return zero.
func processMarketBuyQuantity(filledQty, filledPrice, priceAvg, buyQty fixedpoint.Value, orderStatus v2.OrderStatus) (fixedpoint.Value, error) {
	switch orderStatus {
	case v2.OrderStatusInit, v2.OrderStatusNew, v2.OrderStatusLive, v2.OrderStatusCancelled:
		return fixedpoint.Zero, nil

	case v2.OrderStatusPartialFilled:
		// sanity check for avoid divide 0
		if priceAvg.IsZero() {
			return fixedpoint.Zero, errors.New("priceAvg for a partialFilled should not be zero")
		}
		// calculate the remaining quote coin quantity.
		remainPrice := buyQty.Sub(filledPrice)
		// calculate the remaining base coin quantity.
		remainBaseCoinQty := remainPrice.Div(priceAvg)
		// Estimated quantity that may be purchased.
		return filledQty.Add(remainBaseCoinQty), nil

	case v2.OrderStatusFilled:
		// Market buy orders may not purchase the entire quantity, hence the use of filledQty here.
		return filledQty, nil

	default:
		return fixedpoint.Zero, fmt.Errorf("failed to execute market buy quantity due to unexpected order status %s ", orderStatus)
	}
}

func toLocalOrderType(orderType types.OrderType) (v2.OrderType, error) {
	switch orderType {
	case types.OrderTypeLimit:
		return v2.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return v2.OrderTypeMarket, nil

	default:
		return "", fmt.Errorf("order type %s not supported", orderType)
	}
}

func toLocalSide(side types.SideType) (v2.SideType, error) {
	switch side {
	case types.SideTypeSell:
		return v2.SideTypeSell, nil

	case types.SideTypeBuy:
		return v2.SideTypeBuy, nil

	default:
		return "", fmt.Errorf("side type %s not supported", side)
	}
}

func toGlobalBalanceMap(balances []Balance) types.BalanceMap {
	bm := types.BalanceMap{}
	for _, obj := range balances {
		bm[obj.Coin] = types.Balance{
			Currency:  obj.Coin,
			Available: obj.Available,
			Locked:    obj.Frozen.Add(obj.Locked),
		}
	}
	return bm
}

func toGlobalKLines(symbol string, interval types.Interval, kLines v2.KLineResponse) []types.KLine {
	gKLines := make([]types.KLine, len(kLines))
	for i, kline := range kLines {
		// follow the binance rule, to avoid endTime overlapping with the next startTime. So we subtract -1 time.Millisecond
		// on endTime.
		endTime := types.Time(kline.Ts.Time().Add(interval.Duration() - time.Millisecond))
		gKLines[i] = types.KLine{
			Exchange:    types.ExchangeBitget,
			Symbol:      symbol,
			StartTime:   types.Time(kline.Ts),
			EndTime:     endTime,
			Interval:    interval,
			Open:        kline.Open,
			Close:       kline.Close,
			High:        kline.High,
			Low:         kline.Low,
			Volume:      kline.Volume,
			QuoteVolume: kline.QuoteVolume,
			// Bitget doesn't support close flag in REST API
			Closed: false,
		}
	}
	return gKLines
}
