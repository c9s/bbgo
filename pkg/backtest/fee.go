package backtest

import (
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FeeModeFunction func(order *types.Order, market *types.Market, feeRate fixedpoint.Value) (fee fixedpoint.Value, feeCurrency string)

func feeModeFunctionToken(order *types.Order, _ *types.Market, feeRate fixedpoint.Value) (fee fixedpoint.Value, feeCurrency string) {
	quoteQuantity := order.Quantity.Mul(order.Price)
	feeCurrency = FeeToken
	fee = quoteQuantity.Mul(feeRate)
	return fee, feeCurrency
}

func feeModeFunctionNative(order *types.Order, market *types.Market, feeRate fixedpoint.Value) (fee fixedpoint.Value, feeCurrency string) {
	switch order.Side {

	case types.SideTypeBuy:
		fee = order.Quantity.Mul(feeRate)
		feeCurrency = market.BaseCurrency

	case types.SideTypeSell:
		quoteQuantity := order.Quantity.Mul(order.Price)
		fee = quoteQuantity.Mul(feeRate)
		feeCurrency = market.QuoteCurrency

	}

	return fee, feeCurrency
}

func feeModeFunctionQuote(order *types.Order, market *types.Market, feeRate fixedpoint.Value) (fee fixedpoint.Value, feeCurrency string) {
	feeCurrency = market.QuoteCurrency
	quoteQuantity := order.Quantity.Mul(order.Price)
	fee = quoteQuantity.Mul(feeRate)
	return fee, feeCurrency
}

func getFeeModeFunction(feeMode bbgo.BackTestFeeMode) FeeModeFunction {
	switch feeMode {

	case bbgo.BackTestFeeModeNative:
		return feeModeFunctionNative

	case bbgo.BackTestFeeModeQuote:
		return feeModeFunctionQuote

	case bbgo.BackTestFeeModeToken:
		return feeModeFunctionToken

	default:
		return feeModeFunctionQuote
	}
}
