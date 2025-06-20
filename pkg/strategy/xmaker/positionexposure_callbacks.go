// Code generated by "callbackgen -type PositionExposure"; DO NOT EDIT.

package xmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func (m *PositionExposure) OnCover(cb func(d fixedpoint.Value)) {
	m.coverCallbacks = append(m.coverCallbacks, cb)
}

func (m *PositionExposure) EmitCover(d fixedpoint.Value) {
	for _, cb := range m.coverCallbacks {
		cb(d)
	}
}

func (m *PositionExposure) OnClose(cb func(d fixedpoint.Value)) {
	m.closeCallbacks = append(m.closeCallbacks, cb)
}

func (m *PositionExposure) EmitClose(d fixedpoint.Value) {
	for _, cb := range m.closeCallbacks {
		cb(d)
	}
}
