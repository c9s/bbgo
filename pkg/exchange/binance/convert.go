package binance

import (
	"fmt"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalMarket(symbol binance.Symbol) types.Market {
	market := types.Market{
		Symbol:          symbol.Symbol,
		LocalSymbol:     symbol.Symbol,
		PricePrecision:  symbol.QuotePrecision,
		VolumePrecision: symbol.BaseAssetPrecision,
		QuoteCurrency:   symbol.QuoteAsset,
		BaseCurrency:    symbol.BaseAsset,
	}

	if f := symbol.MinNotionalFilter(); f != nil {
		market.MinNotional = fixedpoint.MustNewFromString(f.MinNotional)
		market.MinAmount = fixedpoint.MustNewFromString(f.MinNotional)
	}

	// The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol.
	// There are 3 parts:
	// minQty defines the minimum quantity/icebergQty allowed.
	//	maxQty defines the maximum quantity/icebergQty allowed.
	//	stepSize defines the intervals that a quantity/icebergQty can be increased/decreased by.
	if f := symbol.LotSizeFilter(); f != nil {
		market.MinQuantity = fixedpoint.MustNewFromString(f.MinQuantity)
		market.MaxQuantity = fixedpoint.MustNewFromString(f.MaxQuantity)
		market.StepSize = fixedpoint.MustNewFromString(f.StepSize)
	}

	if f := symbol.PriceFilter(); f != nil {
		market.MaxPrice = fixedpoint.MustNewFromString(f.MaxPrice)
		market.MinPrice = fixedpoint.MustNewFromString(f.MinPrice)
		market.TickSize = fixedpoint.MustNewFromString(f.TickSize)
	}

	return market
}

// TODO: Cuz it returns types.Market as well, merge following to the above function
func toGlobalFuturesMarket(symbol futures.Symbol) types.Market {
	market := types.Market{
		Symbol:          symbol.Symbol,
		LocalSymbol:     symbol.Symbol,
		PricePrecision:  symbol.QuotePrecision,
		VolumePrecision: symbol.BaseAssetPrecision,
		QuoteCurrency:   symbol.QuoteAsset,
		BaseCurrency:    symbol.BaseAsset,
	}

	if f := symbol.MinNotionalFilter(); f != nil {
		market.MinNotional = fixedpoint.MustNewFromString(f.Notional)
		market.MinAmount = fixedpoint.MustNewFromString(f.Notional)
	}

	// The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol.
	// There are 3 parts:
	// minQty defines the minimum quantity/icebergQty allowed.
	//	maxQty defines the maximum quantity/icebergQty allowed.
	//	stepSize defines the intervals that a quantity/icebergQty can be increased/decreased by.
	if f := symbol.LotSizeFilter(); f != nil {
		market.MinQuantity = fixedpoint.MustNewFromString(f.MinQuantity)
		market.MaxQuantity = fixedpoint.MustNewFromString(f.MaxQuantity)
		market.StepSize = fixedpoint.MustNewFromString(f.StepSize)
	}

	if f := symbol.PriceFilter(); f != nil {
		market.MaxPrice = fixedpoint.MustNewFromString(f.MaxPrice)
		market.MinPrice = fixedpoint.MustNewFromString(f.MinPrice)
		market.TickSize = fixedpoint.MustNewFromString(f.TickSize)
	}

	return market
}

// func toGlobalIsolatedMarginAccount(account *binance.IsolatedMarginAccount) *types.IsolatedMarginAccount {
//	return &types.IsolatedMarginAccount{
//		TotalAssetOfBTC:     fixedpoint.MustNewFromString(account.TotalNetAssetOfBTC),
//		TotalLiabilityOfBTC: fixedpoint.MustNewFromString(account.TotalLiabilityOfBTC),
//		TotalNetAssetOfBTC:  fixedpoint.MustNewFromString(account.TotalNetAssetOfBTC),
//		Assets:              toGlobalIsolatedMarginAssets(account.Assets),
//	}
// }

func toGlobalTicker(stats *binance.PriceChangeStats) (*types.Ticker, error) {
	return &types.Ticker{
		Volume: fixedpoint.MustNewFromString(stats.Volume),
		Last:   fixedpoint.MustNewFromString(stats.LastPrice),
		Open:   fixedpoint.MustNewFromString(stats.OpenPrice),
		High:   fixedpoint.MustNewFromString(stats.HighPrice),
		Low:    fixedpoint.MustNewFromString(stats.LowPrice),
		Buy:    fixedpoint.MustNewFromString(stats.BidPrice),
		Sell:   fixedpoint.MustNewFromString(stats.AskPrice),
		Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
	}, nil
}

func toGlobalFuturesTicker(stats *futures.PriceChangeStats) (*types.Ticker, error) {
	return &types.Ticker{
		Volume: fixedpoint.MustNewFromString(stats.Volume),
		Last:   fixedpoint.MustNewFromString(stats.LastPrice),
		Open:   fixedpoint.MustNewFromString(stats.OpenPrice),
		High:   fixedpoint.MustNewFromString(stats.HighPrice),
		Low:    fixedpoint.MustNewFromString(stats.LowPrice),
		Buy:    fixedpoint.MustNewFromString(stats.LastPrice),
		Sell:   fixedpoint.MustNewFromString(stats.LastPrice),
		Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
	}, nil
}

func toLocalOrderType(orderType types.OrderType) (binance.OrderType, error) {
	switch orderType {

	case types.OrderTypeLimitMaker:
		return binance.OrderTypeLimitMaker, nil

	case types.OrderTypeLimit:
		return binance.OrderTypeLimit, nil

	case types.OrderTypeStopLimit:
		return binance.OrderTypeStopLossLimit, nil

	case types.OrderTypeStopMarket:
		return binance.OrderTypeStopLoss, nil

	case types.OrderTypeMarket:
		return binance.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("can not convert to local order, order type %s not supported", orderType)
}

func toGlobalOrders(binanceOrders []*binance.Order, isMargin bool) (orders []types.Order, err error) {
	for _, binanceOrder := range binanceOrders {
		order, err := toGlobalOrder(binanceOrder, isMargin)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, err
}

func toGlobalOrder(binanceOrder *binance.Order, isMargin bool) (*types.Order, error) {
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: binanceOrder.ClientOrderID,
			Symbol:        binanceOrder.Symbol,
			Side:          toGlobalSideType(binanceOrder.Side),
			Type:          toGlobalOrderType(binanceOrder.Type),
			Quantity:      fixedpoint.MustNewFromString(binanceOrder.OrigQuantity),
			Price:         fixedpoint.MustNewFromString(binanceOrder.Price),
			TimeInForce:   types.TimeInForce(binanceOrder.TimeInForce),
		},
		Exchange:         types.ExchangeBinance,
		IsWorking:        binanceOrder.IsWorking,
		OrderID:          uint64(binanceOrder.OrderID),
		Status:           toGlobalOrderStatus(binanceOrder.Status),
		ExecutedQuantity: fixedpoint.MustNewFromString(binanceOrder.ExecutedQuantity),
		CreationTime:     types.Time(millisecondTime(binanceOrder.Time)),
		UpdateTime:       types.Time(millisecondTime(binanceOrder.UpdateTime)),
		IsMargin:         isMargin,
		IsIsolated:       binanceOrder.IsIsolated,
	}, nil
}

func millisecondTime(t int64) time.Time {
	return time.Unix(0, t*int64(time.Millisecond))
}

func toGlobalTrade(t binance.TradeV3, isMargin bool) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side types.SideType
	if t.IsBuyer {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}

	price, err := fixedpoint.NewFromString(t.Price)
	if err != nil {
		return nil, errors.Wrapf(err, "price parse error, price: %+v", t.Price)
	}

	quantity, err := fixedpoint.NewFromString(t.Quantity)
	if err != nil {
		return nil, errors.Wrapf(err, "quantity parse error, quantity: %+v", t.Quantity)
	}

	var quoteQuantity fixedpoint.Value
	if len(t.QuoteQuantity) > 0 {
		quoteQuantity, err = fixedpoint.NewFromString(t.QuoteQuantity)
		if err != nil {
			return nil, errors.Wrapf(err, "quote quantity parse error, quoteQuantity: %+v", t.QuoteQuantity)
		}
	} else {
		quoteQuantity = price.Mul(quantity)
	}

	fee, err := fixedpoint.NewFromString(t.Commission)
	if err != nil {
		return nil, errors.Wrapf(err, "commission parse error, commission: %+v", t.Commission)
	}

	return &types.Trade{
		ID:            uint64(t.ID),
		OrderID:       uint64(t.OrderID),
		Price:         price,
		Symbol:        t.Symbol,
		Exchange:      "binance",
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		IsBuyer:       t.IsBuyer,
		IsMaker:       t.IsMaker,
		Fee:           fee,
		FeeCurrency:   t.CommissionAsset,
		Time:          types.Time(millisecondTime(t.Time)),
		IsMargin:      isMargin,
		IsIsolated:    t.IsIsolated,
	}, nil
}

func toGlobalSideType(side binance.SideType) types.SideType {
	switch side {
	case binance.SideTypeBuy:
		return types.SideTypeBuy

	case binance.SideTypeSell:
		return types.SideTypeSell

	default:
		log.Errorf("can not convert binance side type, unknown side type: %q", side)
		return ""
	}
}

func toGlobalOrderType(orderType binance.OrderType) types.OrderType {
	switch orderType {

	case binance.OrderTypeLimit,
		binance.OrderTypeLimitMaker, binance.OrderTypeTakeProfitLimit:
		return types.OrderTypeLimit

	case binance.OrderTypeMarket:
		return types.OrderTypeMarket

	case binance.OrderTypeStopLossLimit:
		return types.OrderTypeStopLimit

	case binance.OrderTypeStopLoss:
		return types.OrderTypeStopMarket

	default:
		log.Errorf("unsupported order type: %v", orderType)
		return ""
	}
}

func toGlobalOrderStatus(orderStatus binance.OrderStatusType) types.OrderStatus {
	switch orderStatus {
	case binance.OrderStatusTypeNew:
		return types.OrderStatusNew

	case binance.OrderStatusTypeRejected:
		return types.OrderStatusRejected

	case binance.OrderStatusTypeCanceled, binance.OrderStatusTypeExpired, binance.OrderStatusTypePendingCancel:
		return types.OrderStatusCanceled

	case binance.OrderStatusTypePartiallyFilled:
		return types.OrderStatusPartiallyFilled

	case binance.OrderStatusTypeFilled:
		return types.OrderStatusFilled
	}

	return types.OrderStatus(orderStatus)
}

func convertSubscription(s types.Subscription) string {
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	// for trade, it's "<symbol>@trade"
	// for aggregated trade, it's "<symbol>@aggTrade"
	switch s.Channel {
	case types.KLineChannel:
		return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())
	case types.BookChannel:
		// depth values: 5, 10, 20
		// Stream Names: <symbol>@depth<levels> OR <symbol>@depth<levels>@100ms.
		// Update speed: 1000ms or 100ms
		n := strings.ToLower(s.Symbol) + "@depth"
		switch s.Options.Depth {
		case types.DepthLevel5:
			n += "5"

		case types.DepthLevelMedium:
			n += "20"

		case types.DepthLevelFull:
		default:

		}

		switch s.Options.Speed {
		case types.SpeedHigh:
			n += "@100ms"

		case types.SpeedLow:
			n += "@1000ms"

		}
		return n
	case types.BookTickerChannel:
		return fmt.Sprintf("%s@bookTicker", strings.ToLower(s.Symbol))
	case types.MarketTradeChannel:
		return fmt.Sprintf("%s@trade", strings.ToLower(s.Symbol))
	case types.AggTradeChannel:
		return fmt.Sprintf("%s@aggTrade", strings.ToLower(s.Symbol))
	}

	return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
}
