package binance

import (
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalFuturesAccountInfo(account *binanceapi.FuturesAccount, risks []binanceapi.FuturesPositionRisk) *types.FuturesAccount {
	return &types.FuturesAccount{
		Assets:                      toGlobalFuturesUserAssets(account.Assets),
		Positions:                   toGlobalFuturesPositions(account.Positions, risks),
		TotalInitialMargin:          account.TotalInitialMargin,
		TotalMaintMargin:            account.TotalMaintMargin,
		TotalWalletBalance:          account.TotalWalletBalance,
		TotalMarginBalance:          account.TotalMarginBalance,
		TotalOpenOrderInitialMargin: account.TotalOpenOrderInitialMargin,
		TotalPositionInitialMargin:  account.TotalPositionInitialMargin,
		TotalUnrealizedProfit:       account.TotalUnrealizedProfit,

		AvailableBalance: account.AvailableBalance,
	}
}

func toGlobalFuturesPositions(futuresPositions []binanceapi.FuturesAccountPosition, risks []binanceapi.FuturesPositionRisk) types.FuturesPositionMap {
	riskMap := make(map[types.PositionKey]types.PositionRisk)
	if len(risks) > 0 {
		for _, risk := range toGlobalPositionRisk(risks) {
			riskMap[types.NewPositionKey(risk.Symbol, risk.PositionSide)] = risk
		}
	}

	retFuturesPositions := make(types.FuturesPositionMap)
	for _, futuresPosition := range futuresPositions {
		position := types.FuturesPosition{
			Isolated:     futuresPosition.Isolated,
			AverageCost:  futuresPosition.EntryPrice,
			Base:         futuresPosition.PositionAmt,
			Quote:        futuresPosition.Notional,
			PositionSide: toGlobalPositionSide(futuresPosition.PositionSide),
			Symbol:       futuresPosition.Symbol,
			UpdateTime:   futuresPosition.UpdateTime,
			PositionRisk: &types.PositionRisk{
				Leverage: futuresPosition.Leverage,
			},
		}

		posKey := types.NewPositionKey(futuresPosition.Symbol, position.PositionSide)
		if risk, exists := riskMap[posKey]; exists {
			risk.Leverage = futuresPosition.Leverage
			position.PositionRisk = &risk
		}

		retFuturesPositions[posKey] = position
	}

	return retFuturesPositions
}

func toGlobalFuturesUserAssets(assets []binanceapi.FuturesAccountAsset) (retAssets types.FuturesAssetMap) {
	retFuturesAssets := make(types.FuturesAssetMap)
	for _, futuresAsset := range assets {
		retFuturesAssets[futuresAsset.Asset] = types.FuturesUserAsset{
			Asset:                  futuresAsset.Asset,
			InitialMargin:          futuresAsset.InitialMargin,
			MaintMargin:            futuresAsset.MaintMargin,
			MarginBalance:          futuresAsset.MarginBalance,
			MaxWithdrawAmount:      futuresAsset.MaxWithdrawAmount,
			OpenOrderInitialMargin: futuresAsset.OpenOrderInitialMargin,
			PositionInitialMargin:  futuresAsset.PositionInitialMargin,
			UnrealizedProfit:       futuresAsset.UnrealizedProfit,
			WalletBalance:          futuresAsset.WalletBalance,
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

	case types.OrderTypeStopLimit:
		return futures.OrderTypeStop, nil

	case types.OrderTypeStopMarket:
		return futures.OrderTypeStopMarket, nil

	case types.OrderTypeMarket:
		return futures.OrderTypeMarket, nil

	case types.OrderTypeTakeProfitMarket:
		return futures.OrderTypeTakeProfitMarket, nil
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
	orderPrice := futuresOrder.Price
	if orderPrice == "" {
		orderPrice = futuresOrder.AvgPrice
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: futuresOrder.ClientOrderID,
			Symbol:        futuresOrder.Symbol,
			Side:          toGlobalFuturesSideType(futuresOrder.Side),
			Type:          toGlobalFuturesOrderType(futuresOrder.Type),
			ReduceOnly:    futuresOrder.ReduceOnly,
			ClosePosition: futuresOrder.ClosePosition,
			Quantity:      fixedpoint.MustNewFromString(futuresOrder.OrigQuantity),
			StopPrice:     fixedpoint.MustNewFromString(futuresOrder.StopPrice),
			Price:         fixedpoint.MustNewFromString(orderPrice),
			TimeInForce:   types.TimeInForce(futuresOrder.TimeInForce),
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          uint64(futuresOrder.OrderID),
		Status:           toGlobalFuturesOrderStatus(futuresOrder.Status),
		ExecutedQuantity: fixedpoint.MustNewFromString(futuresOrder.ExecutedQuantity),
		CreationTime:     types.Time(millisecondTime(futuresOrder.Time)),
		UpdateTime:       types.Time(millisecondTime(futuresOrder.UpdateTime)),
		IsFutures:        true,
		IsIsolated:       isIsolated,
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
		return types.OrderTypeTakeProfitMarket

	case futures.OrderTypeStopMarket:
		return types.OrderTypeStopMarket

	case futures.OrderTypeLimit:
		return types.OrderTypeLimit

	case futures.OrderTypeStop:
		return types.OrderTypeStopLimit

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

func toGlobalPositionRisk(positions []binanceapi.FuturesPositionRisk) []types.PositionRisk {
	retPositions := make([]types.PositionRisk, len(positions))
	for i, position := range positions {
		retPositions[i] = types.PositionRisk{
			LiquidationPrice:       position.LiquidationPrice,
			PositionSide:           toGlobalPositionSide(position.PositionSide),
			Symbol:                 position.Symbol,
			MarkPrice:              position.MarkPrice,
			EntryPrice:             position.EntryPrice,
			PositionAmount:         position.PositionAmount,
			BreakEvenPrice:         position.BreakEvenPrice,
			UnrealizedPnL:          position.UnRealizedProfit,
			InitialMargin:          position.InitialMargin,
			Notional:               position.Notional,
			PositionInitialMargin:  position.PositionInitialMargin,
			MaintMargin:            position.MaintMargin,
			Adl:                    position.Adl,
			OpenOrderInitialMargin: position.OpenOrderInitialMargin,
			UpdateTime:             position.UpdateTime,
			MarginAsset:            position.MarginAsset,
		}
	}
	return retPositions
}

func toGlobalPositionSide(positionSide string) types.PositionType {
	switch positionSide {
	case "LONG":
		return types.PositionLong
	case "SHORT":
		return types.PositionShort
	default:
		return types.PositionType(positionSide)
	}
}
