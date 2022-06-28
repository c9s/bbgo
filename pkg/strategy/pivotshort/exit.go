package pivotshort

import "github.com/c9s/bbgo/pkg/bbgo"

type ExitMethod struct {
	RoiStopLoss               *RoiStopLoss               `json:"roiStopLoss"`
	ProtectiveStopLoss        *ProtectiveStopLoss        `json:"protectiveStopLoss"`
	RoiTakeProfit             *RoiTakeProfit             `json:"roiTakeProfit"`
	LowerShadowTakeProfit     *LowerShadowTakeProfit     `json:"lowerShadowTakeProfit"`
	CumulatedVolumeTakeProfit *CumulatedVolumeTakeProfit `json:"cumulatedVolumeTakeProfit"`
}

func (m *ExitMethod) Bind(session *bbgo.ExchangeSession, orderExecutor *bbgo.GeneralOrderExecutor) {
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
