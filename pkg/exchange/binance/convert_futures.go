package binance

import (
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"

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
	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		return futures.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return futures.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("can not convert to local order, order type %s not supported", orderType)
}

// isAlgoOrderType checks if the order type is an Algo order type (conditional order)
func isAlgoOrderType(orderType types.OrderType) bool {
	switch orderType {
	case types.OrderTypeStopLimit,
		types.OrderTypeStopMarket,
		types.OrderTypeTakeProfit,
		types.OrderTypeTakeProfitMarket,
		types.OrderTypeTrailingStopMarket:
		return true
	default:
		return false
	}
}

func toLocalFuturesAlgoOrderType(orderType types.OrderType) (futures.AlgoOrderType, error) {
	switch orderType {
	case types.OrderTypeStopLimit:
		return futures.AlgoOrderTypeStop, nil

	case types.OrderTypeStopMarket:
		return futures.AlgoOrderTypeStopMarket, nil

	case types.OrderTypeTakeProfit:
		return futures.AlgoOrderTypeTakeProfit, nil

	case types.OrderTypeTakeProfitMarket:
		return futures.AlgoOrderTypeTakeProfitMarket, nil
	}

	return "", fmt.Errorf("can not convert to local order, order type %s not supported", orderType)
}

// toGlobalFuturesOrders converts a slice of futures orders (regular or algo) to global orders
func toGlobalFuturesOrders[T FuturesOrderConstraint](futuresOrders []T, isIsolated bool) (orders []types.Order, err error) {
	for _, futuresOrder := range futuresOrders {
		order, err := toGlobalFuturesOrder(futuresOrder, isIsolated)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, nil
}

// FuturesOrderConstraint is a type constraint that limits to *futures.Order, *futures.CreateAlgoOrderResp, or futures.GetAlgoOrderResp
type FuturesOrderConstraint interface {
	*futures.Order | *futures.CreateAlgoOrderResp | futures.GetAlgoOrderResp | *futures.GetAlgoOrderResp
}

// toGlobalFuturesOrder converts both *futures.Order and *futures.CreateAlgoOrderResp to types.Order
func toGlobalFuturesOrder[T FuturesOrderConstraint](orderSource T, isIsolated bool) (*types.Order, error) {
	var (
		clientOrderID    string
		symbol           string
		side             futures.SideType
		orderType        any
		reduceOnly       bool
		closePosition    bool
		origQuantity     string
		stopPrice        string
		price            string
		avgPrice         string
		timeInForce      futures.TimeInForceType
		orderID          int64
		status           futures.OrderStatusType
		executedQuantity string
		time             int64
		updateTime       int64
	)

	// Extract fields based on type
	switch o := any(orderSource).(type) {
	case *futures.Order:
		clientOrderID = o.ClientOrderID
		symbol = o.Symbol
		side = o.Side
		orderType = o.Type
		reduceOnly = o.ReduceOnly
		closePosition = o.ClosePosition
		origQuantity = o.OrigQuantity
		stopPrice = o.StopPrice
		price = o.Price
		avgPrice = o.AvgPrice
		timeInForce = o.TimeInForce
		orderID = o.OrderID
		status = o.Status
		executedQuantity = o.ExecutedQuantity
		time = o.Time
		updateTime = o.UpdateTime
	case *futures.CreateAlgoOrderResp:
		clientOrderID = o.ClientAlgoId
		symbol = o.Symbol
		side = o.Side
		orderType = o.OrderType
		reduceOnly = o.ReduceOnly
		closePosition = o.ClosePosition
		origQuantity = o.Quantity
		stopPrice = o.TriggerPrice
		price = o.Price
		avgPrice = "" // CreateAlgoOrderResp doesn't have AvgPrice
		timeInForce = o.TimeInForce
		orderID = o.AlgoId
		status = futures.OrderStatusType(o.AlgoStatus)
		executedQuantity = "0" // CreateAlgoOrderResp doesn't have ExecutedQuantity
		time = o.CreateTime
		updateTime = o.UpdateTime
	case futures.GetAlgoOrderResp:
	case *futures.GetAlgoOrderResp:
		// GetAlgoOrderResp has the same structure as CreateAlgoOrderResp
		clientOrderID = o.ClientAlgoId
		symbol = o.Symbol
		side = o.Side
		orderType = o.OrderType
		reduceOnly = o.ReduceOnly
		closePosition = o.ClosePosition
		origQuantity = o.Quantity
		stopPrice = o.TriggerPrice
		price = o.Price
		avgPrice = "" // GetAlgoOrderResp doesn't have AvgPrice
		timeInForce = o.TimeInForce
		orderID = o.AlgoId
		status = futures.OrderStatusType(o.AlgoStatus)
		executedQuantity = "0" // GetAlgoOrderResp doesn't have ExecutedQuantity
		time = o.CreateTime
		updateTime = o.UpdateTime
	default:
		return nil, fmt.Errorf("unsupported order source type: %T", orderSource)
	}

	// Use price or avgPrice
	orderPrice := price
	if orderPrice == "" {
		orderPrice = avgPrice
	}

	// Convert order type using type assertion
	var globalOrderType types.OrderType
	switch v := orderType.(type) {
	case futures.OrderType:
		globalOrderType = toGlobalFuturesOrderType(v)
	case futures.AlgoOrderType:
		globalOrderType = toGlobalFuturesOrderType(v)
	default:
		return nil, fmt.Errorf("unsupported order type: %T", orderType)
	}

	return &types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: clientOrderID,
			Symbol:        symbol,
			Side:          toGlobalFuturesSideType(side),
			Type:          globalOrderType,
			ReduceOnly:    reduceOnly,
			ClosePosition: closePosition,
			Quantity:      fixedpoint.MustNewFromString(origQuantity),
			StopPrice:     fixedpoint.MustNewFromString(stopPrice),
			Price:         fixedpoint.MustNewFromString(orderPrice),
			TimeInForce:   types.TimeInForce(timeInForce),
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          uint64(orderID),
		Status:           toGlobalFuturesOrderStatus(status),
		ExecutedQuantity: fixedpoint.MustNewFromString(executedQuantity),
		CreationTime:     types.Time(millisecondTime(time)),
		UpdateTime:       types.Time(millisecondTime(updateTime)),
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

// OrderTypeConstraint is a type constraint that limits to futures.OrderType or futures.AlgoOrderType
type OrderTypeConstraint interface {
	futures.OrderType | futures.AlgoOrderType
}

// toGlobalFuturesOrderType converts both futures.OrderType and futures.AlgoOrderType to types.OrderType
func toGlobalFuturesOrderType[T OrderTypeConstraint](orderType T) types.OrderType {
	orderTypeStr := string(orderType)

	switch orderTypeStr {
	// futures.OrderType values
	case string(futures.OrderTypeLimit):
		return types.OrderTypeLimit

	case string(futures.OrderTypeMarket):
		return types.OrderTypeMarket

	// futures.AlgoOrderType values
	case string(futures.AlgoOrderTypeStop):
		return types.OrderTypeStopLimit

	case string(futures.AlgoOrderTypeStopMarket):
		return types.OrderTypeStopMarket

	case string(futures.AlgoOrderTypeTakeProfit):
		return types.OrderTypeTakeProfit

	case string(futures.AlgoOrderTypeTakeProfitMarket):
		return types.OrderTypeTakeProfitMarket

	case string(futures.AlgoOrderTypeTrailingStopMarket):
		return types.OrderTypeStopMarket

	default:
		log.Errorf("unsupported binance futures order type: %s", orderTypeStr)
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

// toGlobalFuturesAlgoOrder converts the response of CreateAlgoOrderService
// to a generic types.Order model.
func toGlobalFuturesAlgoOrder(resp *futures.CreateAlgoOrderResp) (*types.Order, error) {
	// safe parser: empty string -> 0
	parse := func(s string) fixedpoint.Value {
		if s == "" {
			return fixedpoint.Zero
		}
		v, err := fixedpoint.NewFromString(s)
		if err != nil {
			// be tolerant: return zero on parse error
			return fixedpoint.Zero
		}
		return v
	}

	submit := types.SubmitOrder{
		ClientOrderID: resp.ClientAlgoId,
		Symbol:        resp.Symbol,
		Side:          toGlobalFuturesSideType(resp.Side),
		Type:          toGlobalFuturesAlgoOrderType(resp.OrderType),
		Price:         parse(resp.Price),
		Quantity:      parse(resp.Quantity),
		StopPrice:     parse(resp.TriggerPrice),
		TimeInForce:   types.TimeInForce(resp.TimeInForce),
		ReduceOnly:    resp.ReduceOnly,
		ClosePosition: resp.ClosePosition,
	}

	// Algo create response doesn’t include an actual OrderID until triggered.
	// We construct a pending NEW order placeholder.
	o := &types.Order{
		SubmitOrder:      submit,
		Exchange:         types.ExchangeBinance,
		OrderID:          0,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		CreationTime:     types.Time(millisecondTime(resp.CreateTime)),
		UpdateTime:       types.Time(millisecondTime(resp.UpdateTime)),
		IsFutures:        true,
		IsIsolated:       false,
	}
	return o, nil
}

// toGlobalFuturesAlgoOrderType maps futures.AlgoOrderType to bbgo types.OrderType.
func toGlobalFuturesAlgoOrderType(t futures.AlgoOrderType) types.OrderType {
	switch t {
	case futures.AlgoOrderTypeStopMarket:
		return types.OrderTypeStopMarket
	case futures.AlgoOrderTypeTakeProfitMarket:
		return types.OrderTypeTakeProfitMarket
	case futures.AlgoOrderTypeStop:
		// Treat as stop-limit in our taxonomy
		return types.OrderTypeStopLimit
	case futures.AlgoOrderTypeTakeProfit:
		// Treat as take-profit (limit) if ever used
		return types.OrderType("TAKE_PROFIT")
	case futures.AlgoOrderTypeTrailingStopMarket:
		return types.OrderType("TRAILING_STOP_MARKET")
	default:
		log.Errorf("unsupported binance futures algo order type: %s", t)
		return types.OrderType(t)
	}
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
