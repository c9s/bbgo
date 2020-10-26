package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

type SymbolBasedOrderExecutor struct {
	BasicRiskControlOrderExecutor *BasicRiskControlOrderExecutor `json:"basic,omitempty" yaml:"basic,omitempty"`
}

type RiskControlOrderExecutors struct {
	Notifiability `json:"-"`
	session       *ExchangeSession

	// Symbol => Executor config
	BySymbol map[string]*SymbolBasedOrderExecutor `json:"bySymbol,omitempty" yaml:"bySymbol,omitempty"`
}

func (e *RiskControlOrderExecutors) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) ([]types.Order, error) {
	var symbolOrders = make(map[string][]types.SubmitOrder, len(orders))
	for _, order := range orders {
		symbolOrders[order.Symbol] = append(symbolOrders[order.Symbol], order)
	}

	var retOrders []types.Order
	for symbol, orders := range symbolOrders {
		if exec, ok := e.BySymbol[symbol]; ok && exec.BasicRiskControlOrderExecutor != nil {
			retOrders2, err := exec.BasicRiskControlOrderExecutor.SubmitOrders(ctx, orders...)
			if err != nil {
				return retOrders, err
			}

			retOrders = append(retOrders, retOrders2...)
		}
	}

	return retOrders, nil
}

type SessionBasedRiskControl struct {
	OrderExecutor *RiskControlOrderExecutors `json:"orderExecutors,omitempty" yaml:"orderExecutors"`
}

func (control *SessionBasedRiskControl) SetSession(session *ExchangeSession) {
	if control.OrderExecutor == nil {
		return
	}

	control.OrderExecutor.session = session

	if control.OrderExecutor.BySymbol == nil {
		return
	}

	for _, exec := range control.OrderExecutor.BySymbol {
		if exec.BasicRiskControlOrderExecutor != nil {
			exec.BasicRiskControlOrderExecutor.session = session
		}
	}
}

type RiskControls struct {
	SessionBasedRiskControl map[string]*SessionBasedRiskControl `json:"sessionBased,omitempty" yaml:"sessionBased,omitempty"`
}

func (controls *RiskControls) SetSession(name string, session *ExchangeSession) {
	if controls.SessionBasedRiskControl == nil {
		return
	}

	control, ok := controls.SessionBasedRiskControl[name]
	if !ok {
		return
	}

	control.SetSession(session)
}
