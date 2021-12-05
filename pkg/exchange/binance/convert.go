package binance

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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
		market.MinNotional = util.MustParseFloat(f.MinNotional)
		market.MinAmount = util.MustParseFloat(f.MinNotional)
	}

	// The LOT_SIZE filter defines the quantity (aka "lots" in auction terms) rules for a symbol.
	// There are 3 parts:
	// minQty defines the minimum quantity/icebergQty allowed.
	//	maxQty defines the maximum quantity/icebergQty allowed.
	//	stepSize defines the intervals that a quantity/icebergQty can be increased/decreased by.
	if f := symbol.LotSizeFilter(); f != nil {
		market.MinQuantity = util.MustParseFloat(f.MinQuantity)
		market.MaxQuantity = util.MustParseFloat(f.MaxQuantity)
		market.StepSize = util.MustParseFloat(f.StepSize)
	}

	if f := symbol.PriceFilter(); f != nil {
		market.MaxPrice = util.MustParseFloat(f.MaxPrice)
		market.MinPrice = util.MustParseFloat(f.MinPrice)
		market.TickSize = util.MustParseFloat(f.TickSize)
	}

	return market
}

func toGlobalIsolatedUserAsset(userAsset binance.IsolatedUserAsset) types.IsolatedUserAsset {
	return types.IsolatedUserAsset{
		Asset:         userAsset.Asset,
		Borrowed:      fixedpoint.MustNewFromString(userAsset.Borrowed),
		Free:          fixedpoint.MustNewFromString(userAsset.Free),
		Interest:      fixedpoint.MustNewFromString(userAsset.Interest),
		Locked:        fixedpoint.MustNewFromString(userAsset.Locked),
		NetAsset:      fixedpoint.MustNewFromString(userAsset.NetAsset),
		NetAssetOfBtc: fixedpoint.MustNewFromString(userAsset.NetAssetOfBtc),
		BorrowEnabled: userAsset.BorrowEnabled,
		RepayEnabled:  userAsset.RepayEnabled,
		TotalAsset:    fixedpoint.MustNewFromString(userAsset.TotalAsset),
	}
}
func toGlobalIsolatedMarginAsset(asset binance.IsolatedMarginAsset) types.IsolatedMarginAsset {
	return types.IsolatedMarginAsset{
		Symbol:            asset.Symbol,
		QuoteAsset:        toGlobalIsolatedUserAsset(asset.QuoteAsset),
		BaseAsset:         toGlobalIsolatedUserAsset(asset.BaseAsset),
		IsolatedCreated:   asset.IsolatedCreated,
		MarginLevel:       fixedpoint.MustNewFromString(asset.MarginLevel),
		MarginLevelStatus: asset.MarginLevelStatus,
		MarginRatio:       fixedpoint.MustNewFromString(asset.MarginRatio),
		IndexPrice:        fixedpoint.MustNewFromString(asset.IndexPrice),
		LiquidatePrice:    fixedpoint.MustNewFromString(asset.LiquidatePrice),
		LiquidateRate:     fixedpoint.MustNewFromString(asset.LiquidateRate),
		TradeEnabled:      false,
	}
}

func toGlobalIsolatedMarginAssets(assets []binance.IsolatedMarginAsset) (retAssets []types.IsolatedMarginAsset) {
	for _, asset := range assets {
		retAssets = append(retAssets, toGlobalIsolatedMarginAsset(asset))
	}

	return retAssets
}

func toGlobalIsolatedMarginAccount(account *binance.IsolatedMarginAccount) *types.IsolatedMarginAccount {
	return &types.IsolatedMarginAccount{
		TotalAssetOfBTC:     fixedpoint.MustNewFromString(account.TotalNetAssetOfBTC),
		TotalLiabilityOfBTC: fixedpoint.MustNewFromString(account.TotalLiabilityOfBTC),
		TotalNetAssetOfBTC:  fixedpoint.MustNewFromString(account.TotalNetAssetOfBTC),
		Assets:              toGlobalIsolatedMarginAssets(account.Assets),
	}
}

func toGlobalMarginUserAssets(userAssets []binance.UserAsset) (retAssets []types.MarginUserAsset) {
	for _, asset := range userAssets {
		retAssets = append(retAssets, types.MarginUserAsset{
			Asset:    asset.Asset,
			Borrowed: fixedpoint.MustNewFromString(asset.Borrowed),
			Free:     fixedpoint.MustNewFromString(asset.Free),
			Interest: fixedpoint.MustNewFromString(asset.Interest),
			Locked:   fixedpoint.MustNewFromString(asset.Locked),
			NetAsset: fixedpoint.MustNewFromString(asset.NetAsset),
		})
	}

	return retAssets
}

func toGlobalMarginAccount(account *binance.MarginAccount) *types.MarginAccount {
	return &types.MarginAccount{
		BorrowEnabled:       account.BorrowEnabled,
		MarginLevel:         fixedpoint.MustNewFromString(account.MarginLevel),
		TotalAssetOfBTC:     fixedpoint.MustNewFromString(account.TotalAssetOfBTC),
		TotalLiabilityOfBTC: fixedpoint.MustNewFromString(account.TotalLiabilityOfBTC),
		TotalNetAssetOfBTC:  fixedpoint.MustNewFromString(account.TotalNetAssetOfBTC),
		TradeEnabled:        account.TradeEnabled,
		TransferEnabled:     account.TransferEnabled,
		UserAssets:          toGlobalMarginUserAssets(account.UserAssets),
	}
}

func toGlobalFuturesAccount(account *futures.Account) *types.Account {
	return &types.Account{
		Assets:                      toGlobalFuturesUserAssets(account.Assets),
		CanDeposit:                  account.CanDeposit,
		CanTrade:                    account.CanTrade,
		CanWithdraw:                 account.CanWithdraw,
		FeeTier:                     account.FeeTier,
		MaxWithdrawAmount:           fixedpoint.MustNewFromString(account.MaxWithdrawAmount),
		Positions:                   toGlobalPositions(account.Positions), // TODO
		TotalInitialMargin:          fixedpoint.MustNewFromString(account.TotalInitialMargin),
		TotalMaintMargin:            fixedpoint.MustNewFromString(account.TotalMaintMargin),
		TotalMarginBalance:          fixedpoint.MustNewFromString(account.TotalMarginBalance),
		TotalOpenOrderInitialMargin: fixedpoint.MustNewFromString(account.TotalOpenOrderInitialMargin),
		TotalPositionInitialMargin:  fixedpoint.MustNewFromString(account.TotalPositionInitialMargin),
		TotalUnrealizedProfit:       fixedpoint.MustNewFromString(account.TotalUnrealizedProfit),
		TotalWalletBalance:          fixedpoint.MustNewFromString(account.TotalWalletBalance),
		UpdateTime:                  account.UpdateTime,
	}
}

func toGlobalFuturesUserAssets(assets []*futures.AccountAsset) (retAssets []types.FuturesUserAsset) {
	for _, asset := range assets {
		retAssets = append(retAssets, types.FuturesUserAsset{
			Asset:                  asset.Asset,
			InitialMargin:          fixedpoint.MustNewFromString(asset.InitialMargin),
			MaintMargin:            fixedpoint.MustNewFromString(asset.MaintMargin),
			MarginBalance:          fixedpoint.MustNewFromString(asset.MarginBalance),
			MaxWithdrawAmount:      fixedpoint.MustNewFromString(asset.MaxWithdrawAmount),
			OpenOrderInitialMargin: fixedpoint.MustNewFromString(asset.OpenOrderInitialMargin),
			PositionInitialMargin:  fixedpoint.MustNewFromString(asset.PositionInitialMargin),
			UnrealizedProfit:       fixedpoint.MustNewFromString(asset.UnrealizedProfit),
			WalletBalance:          fixedpoint.MustNewFromString(asset.WalletBalance),
		})
	}

	return retAssets
}

func toGlobalPositions(positions []*futures.AccountPosition) types.PositionMap {
	retPositions := make(types.PositionMap)
	for _, position := range positions {
		retPositions[position.Symbol] = types.Position{
			Isolated:               position.Isolated,
			Leverage:               fixedpoint.MustNewFromString(position.Leverage),
			InitialMargin:          fixedpoint.MustNewFromString(position.InitialMargin),
			MaintMargin:            fixedpoint.MustNewFromString(position.MaintMargin),
			OpenOrderInitialMargin: fixedpoint.MustNewFromString(position.OpenOrderInitialMargin),
			PositionInitialMargin:  fixedpoint.MustNewFromString(position.PositionInitialMargin),
			Symbol:                 position.Symbol,
			UnrealizedProfit:       fixedpoint.MustNewFromString(position.UnrealizedProfit),
			EntryPrice:             fixedpoint.MustNewFromString(position.EntryPrice),
			MaxNotional:            fixedpoint.MustNewFromString(position.MaxNotional),
			PositionSide:           string(position.PositionSide),
			PositionAmt:            fixedpoint.MustNewFromString(position.PositionAmt),
			Notional:               fixedpoint.MustNewFromString(position.Notional),
			IsolatedWallet:         fixedpoint.MustNewFromString(position.IsolatedWallet),
			UpdateTime:             position.UpdateTime,
		}
	}

	return retPositions
}

// Isolated
// func toGlobalIsolatedFuturesAccount(account *futures.Account) *types.Account {
// 	return &types.IsolatedFuturesAccount{
// 		Assets:                      toGlobalIsolatedFuturesUserAssets(account.Assets),
// 		CanDeposit:                  account.CanDeposit,
// 		CanTrade:                    account.CanTrade,
// 		CanWithdraw:                 account.CanWithdraw,
// 		FeeTier:                     account.FeeTier,
// 		MaxWithdrawAmount:           fixedpoint.MustNewFromString(account.MaxWithdrawAmount),
// 		Positions:                   toGlobalIsolatedFuturesUserPositions(account.Positions),//TODO
// 		TotalInitialMargin:          fixedpoint.MustNewFromString(account.TotalInitialMargin),
// 		TotalMaintMargin:            fixedpoint.MustNewFromString(account.TotalMaintMargin),
// 		TotalMarginBalance:          fixedpoint.MustNewFromString(account.TotalMarginBalance),
// 		TotalOpenOrderInitialMargin: fixedpoint.MustNewFromString(account.TotalOpenOrderInitialMargin),
// 		TotalPositionInitialMargin:  fixedpoint.MustNewFromString(account.TotalPositionInitialMargin),
// 		TotalUnrealizedProfit:       fixedpoint.MustNewFromString(account.TotalUnrealizedProfit),
// 		TotalWalletBalance:          fixedpoint.MustNewFromString(account.TotalWalletBalance),
// 		UpdateTime:                  account.UpdateTime,
// 	}
// }

// func toGlobalIsolatedFuturesUserAssets(userAssets []*futures.AccountAsset) (retAssets []types.IsolatedFuturesUserAsset) {
// 	for _, asset := range userAssets {
// 		retAssets = append(retAssets, types.IsolatedFuturesUserAsset{
// 			Asset:				asset.Asset,
// 			InitialMargin:		fixedpoint.MustNewFromString(asset.InitialMargin),
// 			MaintMargin:            fixedpoint.MustNewFromString(asset.MaintMargin),
// 			MarginBalance:          fixedpoint.MustNewFromString(asset.MarginBalance),
// 			MaxWithdrawAmount:      fixedpoint.MustNewFromString(asset.MaxWithdrawAmount),
// 			OpenOrderInitialMargin: fixedpoint.MustNewFromString(asset.OpenOrderInitialMargin),
// 			PositionInitialMargin:  fixedpoint.MustNewFromString(asset.PositionInitialMargin),
// 			UnrealizedProfit:       fixedpoint.MustNewFromString(asset.UnrealizedProfit),
// 			WalletBalance:         fixedpoint.MustNewFromString(asset.WalletBalance),
// 		})
// 	}

// 	return retAssets
// }

// func toGlobalIsolatedFuturesUserPositions(userPositions []*futures.AccountPosition) (retAssets []bbgo.Position) {
// 	for _, position := range userPositions {
// 		retAssets = append(retAssets, types.IsolatedFuturesUserPosition{
// 			Isolated:               position.Isolated,
// 			Leverage:               fixedpoint.MustNewFromString(position.Leverage),
// 			InitialMargin:          fixedpoint.MustNewFromString(position.InitialMargin),
// 			MaintMargin:            fixedpoint.MustNewFromString(position.MaintMargin),
// 			OpenOrderInitialMargin: fixedpoint.MustNewFromString(position.OpenOrderInitialMargin),
// 			PositionInitialMargin:  fixedpoint.MustNewFromString(position.PositionInitialMargin),
// 			Symbol:                 position.Symbol,
// 			UnrealizedProfit:       fixedpoint.MustNewFromString(position.UnrealizedProfit),
// 			EntryPrice:             fixedpoint.MustNewFromString(position.EntryPrice),
// 			MaxNotional:            fixedpoint.MustNewFromString(position.MaxNotional),
// 			PositionSide:           string(position.PositionSide),
// 			PositionAmt:            fixedpoint.MustNewFromString(position.PositionAmt),
// 			Notional:               fixedpoint.MustNewFromString(position.Notional),
// 			IsolatedWallet:         fixedpoint.MustNewFromString(position.IsolatedWallet),
// 			UpdateTime:             position.UpdateTime,
// 		})
// 	}

// 	return retAssets
// }

func toGlobalTicker(stats *binance.PriceChangeStats) (*types.Ticker, error) {
	return &types.Ticker{
		Volume: util.MustParseFloat(stats.Volume),
		Last:   util.MustParseFloat(stats.LastPrice),
		Open:   util.MustParseFloat(stats.OpenPrice),
		High:   util.MustParseFloat(stats.HighPrice),
		Low:    util.MustParseFloat(stats.LowPrice),
		Buy:    util.MustParseFloat(stats.BidPrice),
		Sell:   util.MustParseFloat(stats.AskPrice),
		Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
	}, nil
}

func toGlobalFuturesTicker(stats *futures.PriceChangeStats) types.Ticker {
	return types.Ticker{
		Volume: util.MustParseFloat(stats.Volume),
		Last:   util.MustParseFloat(stats.LastPrice),
		Open:   util.MustParseFloat(stats.OpenPrice),
		High:   util.MustParseFloat(stats.HighPrice),
		Low:    util.MustParseFloat(stats.LowPrice),
		// Buy:    util.MustParseFloat(stats.BidPrice),
		// Sell:   util.MustParseFloat(stats.AskPrice),
		Time: time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
	}
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

func toLocalFuturesOrderType(orderType types.OrderType) (futures.OrderType, error) {
	switch orderType {

	// case types.OrderTypeLimitMaker:
	// 	return futures.OrderTypeLimitMaker, nil //TODO

	case types.OrderTypeLimit:
		return futures.OrderTypeLimit, nil

	// case types.OrderTypeStopLimit:
	// 	return futures.OrderTypeStopLossLimit, nil //TODO

	// case types.OrderTypeStopMarket:
	// 	return futures.OrderTypeStopLoss, nil //TODO

	case types.OrderTypeMarket:
		return futures.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("can not convert to local order, order type %s not supported", orderType)
}

func toGlobalOrders(binanceOrders []*binance.Order) (orders []types.Order, err error) {
	for _, binanceOrder := range binanceOrders {
		order, err := toGlobalOrder(binanceOrder, false)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, err
}

func toGlobalFuturesOrders(futuresOrders []*futures.Order) (orders []types.Order, err error) {
	for _, futuresOrder := range futuresOrders {
		order, err := toGlobalFuturesOrder(futuresOrder, false)
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
			Quantity:      util.MustParseFloat(binanceOrder.OrigQuantity),
			Price:         util.MustParseFloat(binanceOrder.Price),
			TimeInForce:   string(binanceOrder.TimeInForce),
		},
		Exchange:         types.ExchangeBinance,
		IsWorking:        binanceOrder.IsWorking,
		OrderID:          uint64(binanceOrder.OrderID),
		Status:           toGlobalOrderStatus(binanceOrder.Status),
		ExecutedQuantity: util.MustParseFloat(binanceOrder.ExecutedQuantity),
		CreationTime:     types.Time(millisecondTime(binanceOrder.Time)),
		UpdateTime:       types.Time(millisecondTime(binanceOrder.UpdateTime)),
		IsMargin:         isMargin,
		IsIsolated:       binanceOrder.IsIsolated,
	}, nil
}

func toGlobalFuturesOrder(futuresOrder *futures.Order, isMargin bool) (*types.Order, error) {
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: futuresOrder.ClientOrderID,
			Symbol:        futuresOrder.Symbol,
			Side:          toGlobalFuturesSideType(futuresOrder.Side),
			Type:          toGlobalFuturesOrderType(futuresOrder.Type),
			ReduceOnly:    futuresOrder.ReduceOnly,
			ClosePosition: futuresOrder.ClosePosition,
			Quantity:      util.MustParseFloat(futuresOrder.OrigQuantity),
			Price:         util.MustParseFloat(futuresOrder.Price),
			TimeInForce:   string(futuresOrder.TimeInForce),
		},
		Exchange: types.ExchangeBinance,
		// IsWorking:        futuresOrder.IsWorking,
		OrderID:          uint64(futuresOrder.OrderID),
		Status:           toGlobalFuturesOrderStatus(futuresOrder.Status),
		ExecutedQuantity: util.MustParseFloat(futuresOrder.ExecutedQuantity),
		CreationTime:     types.Time(millisecondTime(futuresOrder.Time)),
		UpdateTime:       types.Time(millisecondTime(futuresOrder.UpdateTime)),
		IsMargin:         isMargin,
		// IsIsolated:       futuresOrder.IsIsolated,
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

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "price parse error, price: %+v", t.Price)
	}

	quantity, err := strconv.ParseFloat(t.Quantity, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "quantity parse error, quantity: %+v", t.Quantity)
	}

	var quoteQuantity = 0.0
	if len(t.QuoteQuantity) > 0 {
		quoteQuantity, err = strconv.ParseFloat(t.QuoteQuantity, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "quote quantity parse error, quoteQuantity: %+v", t.QuoteQuantity)
		}
	} else {
		quoteQuantity = price * quantity
	}

	fee, err := strconv.ParseFloat(t.Commission, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "commission parse error, commission: %+v", t.Commission)
	}

	return &types.Trade{
		ID:            t.ID,
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

func toGlobalFuturesTrade(t futures.AccountTrade) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side types.SideType
	if t.Buyer {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "price parse error, price: %+v", t.Price)
	}

	quantity, err := strconv.ParseFloat(t.Quantity, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "quantity parse error, quantity: %+v", t.Quantity)
	}

	var quoteQuantity = 0.0
	if len(t.QuoteQuantity) > 0 {
		quoteQuantity, err = strconv.ParseFloat(t.QuoteQuantity, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "quote quantity parse error, quoteQuantity: %+v", t.QuoteQuantity)
		}
	} else {
		quoteQuantity = price * quantity
	}

	fee, err := strconv.ParseFloat(t.Commission, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "commission parse error, commission: %+v", t.Commission)
	}

	return &types.Trade{
		ID:            t.ID,
		OrderID:       uint64(t.OrderID),
		Price:         price,
		Symbol:        t.Symbol,
		Exchange:      "binance",
		Quantity:      quantity,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		IsBuyer:       t.Buyer,
		IsMaker:       t.Maker,
		Fee:           fee,
		FeeCurrency:   t.CommissionAsset,
		Time:          types.Time(millisecondTime(t.Time)),
		IsFutures:     true,
		// IsIsolated:    t.IsIsolated,
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

func toGlobalFuturesSideType(side futures.SideType) types.SideType {
	switch side {
	case futures.SideTypeBuy:
		return types.SideTypeBuy

	case futures.SideTypeSell:
		return types.SideTypeSell

	default:
		log.Errorf("can not convert futures side type, unknown side type: %q", side)
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

func toGlobalFuturesOrderType(orderType futures.OrderType) types.OrderType {
	switch orderType {
	// TODO
	case futures.OrderTypeLimit: // , futures.OrderTypeLimitMaker, futures.OrderTypeTakeProfitLimit:
		return types.OrderTypeLimit

	case futures.OrderTypeMarket:
		return types.OrderTypeMarket
	// TODO
	// case futures.OrderTypeStopLossLimit:
	// 	return types.OrderTypeStopLimit
	// TODO
	// case futures.OrderTypeStopLoss:
	// 	return types.OrderTypeStopMarket

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

	case binance.OrderStatusTypeCanceled:
		return types.OrderStatusCanceled

	case binance.OrderStatusTypePartiallyFilled:
		return types.OrderStatusPartiallyFilled

	case binance.OrderStatusTypeFilled:
		return types.OrderStatusFilled
	}

	return types.OrderStatus(orderStatus)
}

func toGlobalFuturesOrderStatus(orderStatus futures.OrderStatusType) types.OrderStatus {
	switch orderStatus {
	case futures.OrderStatusTypeNew:
		return types.OrderStatusNew

	case futures.OrderStatusTypeRejected:
		return types.OrderStatusRejected

	case futures.OrderStatusTypeCanceled:
		return types.OrderStatusCanceled

	case futures.OrderStatusTypePartiallyFilled:
		return types.OrderStatusPartiallyFilled

	case futures.OrderStatusTypeFilled:
		return types.OrderStatusFilled
	}

	return types.OrderStatus(orderStatus)
}

// ConvertTrades converts the binance v3 trade into the global trade type
func ConvertTrades(remoteTrades []*binance.TradeV3) (trades []types.Trade, err error) {
	for _, t := range remoteTrades {
		trade, err := toGlobalTrade(*t, false)
		if err != nil {
			return nil, errors.Wrapf(err, "binance v3 trade parse error, trade: %+v", *t)
		}

		trades = append(trades, *trade)
	}

	return trades, err
}

// ConvertTrades converts the futures v3 trade into the global trade type
func ConvertFuturesTrades(remoteTrades []*futures.AccountTrade) (trades []types.Trade, err error) {
	for _, t := range remoteTrades {
		trade, err := toGlobalFuturesTrade(*t)
		if err != nil {
			return nil, errors.Wrapf(err, "futures v3 trade parse error, trade: %+v", *t)
		}

		trades = append(trades, *trade)
	}

	return trades, err
}

func convertSubscription(s types.Subscription) string {
	// binance uses lower case symbol name,
	// for kline, it's "<symbol>@kline_<interval>"
	// for depth, it's "<symbol>@depth OR <symbol>@depth@100ms"
	switch s.Channel {
	case types.KLineChannel:
		return fmt.Sprintf("%s@%s_%s", strings.ToLower(s.Symbol), s.Channel, s.Options.String())

	case types.BookChannel:
		return fmt.Sprintf("%s@depth", strings.ToLower(s.Symbol))
	}

	return fmt.Sprintf("%s@%s", strings.ToLower(s.Symbol), s.Channel)
}

func convertPremiumIndex(index *futures.PremiumIndex) (*types.PremiumIndex, error) {
	markPrice, err := fixedpoint.NewFromString(index.MarkPrice)
	if err != nil {
		return nil, err
	}

	lastFundingRate, err := fixedpoint.NewFromString(index.LastFundingRate)
	if err != nil {
		return nil, err
	}

	nextFundingTime := time.Unix(0, index.NextFundingTime*int64(time.Millisecond))
	t := time.Unix(0, index.Time*int64(time.Millisecond))

	return &types.PremiumIndex{
		Symbol:          index.Symbol,
		MarkPrice:       markPrice,
		NextFundingTime: nextFundingTime,
		LastFundingRate: lastFundingRate,
		Time:            t,
	}, nil
}
