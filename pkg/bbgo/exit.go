package bbgo

import (
	"reflect"

	"github.com/c9s/bbgo/pkg/types"
)

type ExitMethod struct {
	RoiStopLoss               *RoiStopLoss               `json:"roiStopLoss"`
	ProtectiveStopLoss        *ProtectiveStopLoss        `json:"protectiveStopLoss"`
	RoiTakeProfit             *RoiTakeProfit             `json:"roiTakeProfit"`
	LowerShadowTakeProfit     *LowerShadowTakeProfit     `json:"lowerShadowTakeProfit"`
	CumulatedVolumeTakeProfit *CumulatedVolumeTakeProfit `json:"cumulatedVolumeTakeProfit"`
}

func (m *ExitMethod) Subscribe() {
	rv := reflect.ValueOf(m).Elem()
	rt := reflect.TypeOf(m)
	rt = rt.Elem()
	for i := 0; i <= rt.NumField(); i++ {
		fieldType := rt.Field(i)
		if fieldType.Type.Implements(reflect.TypeOf((*types.Subscriber)(nil))) {
			method := rv.Field(i).MethodByName("Subscribe")
			method.Call(nil)
		}
	}
}

func (m *ExitMethod) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	if m.ProtectiveStopLoss != nil {
		m.ProtectiveStopLoss.Bind(session, orderExecutor)
	} else if m.RoiStopLoss != nil {
		m.RoiStopLoss.Bind(session, orderExecutor)
	} else if m.RoiTakeProfit != nil {
		m.RoiTakeProfit.Bind(session, orderExecutor)
	} else if m.LowerShadowTakeProfit != nil {
		m.LowerShadowTakeProfit.Bind(session, orderExecutor)
	} else if m.CumulatedVolumeTakeProfit != nil {
		m.CumulatedVolumeTakeProfit.Bind(session, orderExecutor)
	}
}
