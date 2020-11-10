package bbgo

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

type SymbolBasedRiskController struct {
	BasicRiskController *BasicRiskController `json:"basic,omitempty" yaml:"basic,omitempty"`
}

type RiskControlOrderExecutor struct {
	*ExchangeOrderExecutor

	// Symbol => Executor config
	BySymbol map[string]*SymbolBasedRiskController `json:"bySymbol,omitempty" yaml:"bySymbol,omitempty"`
}

func (e *RiskControlOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (retOrders types.OrderSlice, err error) {
	var symbolOrders = groupSubmitOrdersBySymbol(orders)
	for symbol, orders := range symbolOrders {
		if controller, ok := e.BySymbol[symbol]; ok && controller != nil {
			var riskErrs []error

			orders, riskErrs = controller.BasicRiskController.ProcessOrders(e.session, orders...)
			for _, riskErr := range riskErrs {
				// use logger from ExchangeOrderExecutor
				e.logger.Warnf(riskErr.Error())
				logrus.Warnf("RISK ERROR: %s", riskErr.Error())
			}
		}

		formattedOrders, err := formatOrders(e.session, orders)
		if err != nil {
			return retOrders, err
		}

		for _, fo := range formattedOrders {
			logrus.Infof("submit order: %s", fo.String())
		}

		retOrders2, err := e.ExchangeOrderExecutor.SubmitOrders(ctx, formattedOrders...)
		if err != nil {
			return retOrders, err
		}

		retOrders = append(retOrders, retOrders2...)
	}

	return
}

type SessionBasedRiskControl struct {
	OrderExecutor *RiskControlOrderExecutor `json:"orderExecutor,omitempty" yaml:"orderExecutor"`
}

func (control *SessionBasedRiskControl) SetBaseOrderExecutor(executor *ExchangeOrderExecutor) {
	if control.OrderExecutor == nil {
		return
	}

	control.OrderExecutor.ExchangeOrderExecutor = executor
}

func groupSubmitOrdersBySymbol(orders []types.SubmitOrder) map[string][]types.SubmitOrder {
	var symbolOrders = make(map[string][]types.SubmitOrder, len(orders))
	for _, order := range orders {
		symbolOrders[order.Symbol] = append(symbolOrders[order.Symbol], order)
	}

	return symbolOrders
}

type RiskControls struct {
	SessionBasedRiskControl map[string]*SessionBasedRiskControl `json:"sessionBased,omitempty" yaml:"sessionBased,omitempty"`
}
