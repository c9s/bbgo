package bbgo

import (
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/dynamic"
)

type ExitMethodSet []ExitMethod

func (s *ExitMethodSet) SetAndSubscribe(session *ExchangeSession, parent interface{}) {
	for i := range *s {
		m := (*s)[i]

		// manually inherit configuration from strategy
		m.Inherit(parent)
		m.Subscribe(session)
	}
}

type ExitMethod struct {
	RoiStopLoss               *RoiStopLoss               `json:"roiStopLoss"`
	ProtectiveStopLoss        *ProtectiveStopLoss        `json:"protectiveStopLoss"`
	RoiTakeProfit             *RoiTakeProfit             `json:"roiTakeProfit"`
	LowerShadowTakeProfit     *LowerShadowTakeProfit     `json:"lowerShadowTakeProfit"`
	CumulatedVolumeTakeProfit *CumulatedVolumeTakeProfit `json:"cumulatedVolumeTakeProfit"`
	TrailingStop              *TrailingStop2             `json:"trailingStop"`
}

// Inherit is used for inheriting properties from the given strategy struct
// for example, some exit method requires the default interval and symbol name from the strategy param object
func (m *ExitMethod) Inherit(parent interface{}) {
	dynamic.StructFieldsInherit(m, parent)
}

func (m *ExitMethod) Subscribe(session *ExchangeSession) {
	if err := dynamic.CallStructFieldsMethod(m, "Subscribe", session); err != nil {
		panic(errors.Wrap(err, "dynamic Subscribe call failed"))
	}
}

func (m *ExitMethod) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	if m.ProtectiveStopLoss != nil {
		m.ProtectiveStopLoss.Bind(session, orderExecutor)
	}

	if m.RoiStopLoss != nil {
		m.RoiStopLoss.Bind(session, orderExecutor)
	}

	if m.RoiTakeProfit != nil {
		m.RoiTakeProfit.Bind(session, orderExecutor)
	}

	if m.LowerShadowTakeProfit != nil {
		m.LowerShadowTakeProfit.Bind(session, orderExecutor)
	}

	if m.CumulatedVolumeTakeProfit != nil {
		m.CumulatedVolumeTakeProfit.Bind(session, orderExecutor)
	}
}
