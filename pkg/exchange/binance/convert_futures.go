package binance

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalFuturesAccountInfo(account *binanceapi.FuturesAccount) *types.FuturesAccountInfo {
	return &types.FuturesAccountInfo{
		Assets:                      toGlobalFuturesUserAssets(account.Assets),
		Positions:                   toGlobalFuturesPositions(account.Positions),
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

func toGlobalFuturesBalance(balances []*futures.Balance) types.BalanceMap {
	retBalances := make(types.BalanceMap)
	for _, balance := range balances {
		retBalances[balance.Asset] = types.Balance{
			Currency:  balance.Asset,
			Available: fixedpoint.MustNewFromString(balance.AvailableBalance),
		}
	}
	return retBalances
}

func toGlobalFuturesPositions(futuresPositions []*binanceapi.FuturesAccountPosition) types.FuturesPositionMap {
	retFuturesPositions := make(types.FuturesPositionMap)
	for _, futuresPosition := range futuresPositions {
		retFuturesPositions[futuresPosition.Symbol] = types.FuturesPosition{ // TODO: types.FuturesPosition
			Isolated:               futuresPosition.Isolated,
			AverageCost:            fixedpoint.MustNewFromString(futuresPosition.EntryPrice),
			ApproximateAverageCost: fixedpoint.MustNewFromString(futuresPosition.EntryPrice),
			Base:                   fixedpoint.MustNewFromString(futuresPosition.PositionAmt),
			Quote:                  fixedpoint.MustNewFromString(futuresPosition.Notional),

			PositionRisk: &types.PositionRisk{
				Leverage: fixedpoint.MustNewFromString(futuresPosition.Leverage),
			},
			Symbol:     futuresPosition.Symbol,
			UpdateTime: futuresPosition.UpdateTime,
		}
	}

	return retFuturesPositions
}

func toGlobalFuturesUserAssets(assets []*binanceapi.FuturesAccountAsset) (retAssets types.FuturesAssetMap) {
	retFuturesAssets := make(types.FuturesAssetMap)
	for _, futuresAsset := range assets {
		retFuturesAssets[futuresAsset.Asset] = types.FuturesUserAsset{
			Asset:                  futuresAsset.Asset,
			InitialMargin:          fixedpoint.MustNewFromString(futuresAsset.InitialMargin),
			MaintMargin:            fixedpoint.MustNewFromString(futuresAsset.MaintMargin),
			MarginBalance:          fixedpoint.MustNewFromString(futuresAsset.MarginBalance),
			MaxWithdrawAmount:      fixedpoint.MustNewFromString(futuresAsset.MaxWithdrawAmount),
			OpenOrderInitialMargin: fixedpoint.MustNewFromString(futuresAsset.OpenOrderInitialMargin),
			PositionInitialMargin:  fixedpoint.MustNewFromString(futuresAsset.PositionInitialMargin),
			UnrealizedProfit:       fixedpoint.MustNewFromString(futuresAsset.UnrealizedProfit),
			WalletBalance:          fixedpoint.MustNewFromString(futuresAsset.WalletBalance),
		}
	}

	return retFuturesAssets
}

func toLocalFuturesOrderType(orderType types.OrderType) (futures.OrderType, error) {
	switch orderType {

	// case types.OrderTypeLimitMaker:
	// 	return futures.OrderTypeLimitMaker, nil //TODO

	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
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

func toGlobalFuturesOrders(futuresOrders []*futures.Order, isIsolated bool) (orders []types.Order, err error) {
	for _, futuresOrder := range futuresOrders {
		order, err := toGlobalFuturesOrder(futuresOrder, isIsolated)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, err
}

func toGlobalFuturesOrder(futuresOrder *futures.Order, isIsolated bool) (*types.Order, error) {
	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: futuresOrder.ClientOrderID,
			Symbol:        futuresOrder.Symbol,
			Side:          toGlobalFuturesSideType(futuresOrder.Side),
			Type:          toGlobalFuturesOrderType(futuresOrder.Type),
			ReduceOnly:    futuresOrder.ReduceOnly,
			ClosePosition: futuresOrder.ClosePosition,
			Quantity:      fixedpoint.MustNewFromString(futuresOrder.OrigQuantity),
			Price:         fixedpoint.MustNewFromString(futuresOrder.Price),
			TimeInForce:   types.TimeInForce(futuresOrder.TimeInForce),
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          uint64(futuresOrder.OrderID),
		Status:           toGlobalFuturesOrderStatus(futuresOrder.Status),
		ExecutedQuantity: fixedpoint.MustNewFromString(futuresOrder.ExecutedQuantity),
		CreationTime:     types.Time(millisecondTime(futuresOrder.Time)),
		UpdateTime:       types.Time(millisecondTime(futuresOrder.UpdateTime)),
		IsFutures:        true,
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
		IsBuyer:       t.Buyer,
		IsMaker:       t.Maker,
		Fee:           fee,
		FeeCurrency:   t.CommissionAsset,
		Time:          types.Time(millisecondTime(t.Time)),
		IsFutures:     true,
	}, nil
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

func toGlobalFuturesOrderType(orderType futures.OrderType) types.OrderType {
	switch orderType {
	// FIXME: handle this order type
	// case futures.OrderTypeTrailingStopMarket:

	case futures.OrderTypeTakeProfit:
		return types.OrderTypeStopLimit

	case futures.OrderTypeTakeProfitMarket:
		return types.OrderTypeStopMarket

	case futures.OrderTypeStopMarket:
		return types.OrderTypeStopMarket

	case futures.OrderTypeLimit:
		return types.OrderTypeLimit

	case futures.OrderTypeMarket:
		return types.OrderTypeMarket

	default:
		log.Errorf("unsupported binance futures order type: %s", orderType)
		return ""
	}
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

func convertPositionRisk(risk *futures.PositionRisk) (*types.PositionRisk, error) {
	leverage, err := fixedpoint.NewFromString(risk.Leverage)
	if err != nil {
		return nil, err
	}

	liquidationPrice, err := fixedpoint.NewFromString(risk.LiquidationPrice)
	if err != nil {
		return nil, err
	}

	return &types.PositionRisk{
		Leverage:         leverage,
		LiquidationPrice: liquidationPrice,
	}, nil
}
