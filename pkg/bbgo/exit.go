package bbgo

import (
	"reflect"

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

func (s *ExitMethodSet) Bind(session *ExchangeSession, orderExecutor *GeneralOrderExecutor) {
	for _, method := range *s {
		method.Bind(session, orderExecutor)
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
	// we need to pass some information from the strategy configuration to the exit methods, like symbol, interval and window
	rt := reflect.TypeOf(m).Elem()
	rv := reflect.ValueOf(m).Elem()
	for j := 0; j < rv.NumField(); j++ {
		if !rt.Field(j).IsExported() {
			continue
		}

		fieldValue := rv.Field(j)
		if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
			continue
		}

		dynamic.InheritStructValues(fieldValue.Interface(), parent)
	}
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
