// Code generated by "callbackgen -type MACD"; DO NOT EDIT.

package indicator

import ()

func (inc *MACD) OnUpdate(cb func(value float64)) {
	inc.UpdateCallbacks = append(inc.UpdateCallbacks, cb)
}

func (inc *MACD) EmitUpdate(value float64) {
	for _, cb := range inc.UpdateCallbacks {
		cb(value)
	}
}
