package bbgo

import (
	"bytes"
	"encoding/json"
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
	TrailingStop              *TrailingStop2             `json:"trailingStop"`

	// Exit methods for short positions
	// =================================================
	LowerShadowTakeProfit     *LowerShadowTakeProfit     `json:"lowerShadowTakeProfit"`
	CumulatedVolumeTakeProfit *CumulatedVolumeTakeProfit `json:"cumulatedVolumeTakeProfit"`
	SupportTakeProfit         *SupportTakeProfit       `json:"supportTakeProfit"`
}

func (e ExitMethod) String() string {
	var buf bytes.Buffer
	if e.RoiStopLoss != nil {
		b, _ := json.Marshal(e.RoiStopLoss)
		buf.WriteString("roiStopLoss: " + string(b) + ", ")
	}

	if e.ProtectiveStopLoss != nil {
		b, _ := json.Marshal(e.ProtectiveStopLoss)
		buf.WriteString("protectiveStopLoss: " + string(b) + ", ")
	}

	if e.RoiTakeProfit != nil {
		b, _ := json.Marshal(e.RoiTakeProfit)
		buf.WriteString("rioTakeProft: " + string(b) + ", ")
	}

	if e.LowerShadowTakeProfit != nil {
		b, _ := json.Marshal(e.LowerShadowTakeProfit)
		buf.WriteString("lowerShadowTakeProft: " + string(b) + ", ")
	}

	if e.CumulatedVolumeTakeProfit != nil {
		b, _ := json.Marshal(e.CumulatedVolumeTakeProfit)
		buf.WriteString("cumulatedVolumeTakeProfit: " + string(b) + ", ")
	}

	if e.TrailingStop != nil {
		b, _ := json.Marshal(e.TrailingStop)
		buf.WriteString("trailingStop: " + string(b) + ", ")
	}

	if e.SupportTakeProfit != nil {
		b, _ := json.Marshal(e.SupportTakeProfit)
		buf.WriteString("supportTakeProfit: " + string(b) + ", ")
	}

	return buf.String()
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

	if m.SupportTakeProfit != nil {
		m.SupportTakeProfit.Bind(session, orderExecutor)
	}

	if m.TrailingStop != nil {
		m.TrailingStop.Bind(session, orderExecutor)
	}
}
