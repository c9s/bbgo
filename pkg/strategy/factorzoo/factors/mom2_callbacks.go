// Code generated by "callbackgen -type MOM2"; DO NOT EDIT.

package factorzoo

import ()

func (inc *MOM2) OnUpdate(cb func(val float64)) {
	inc.UpdateCallbacks = append(inc.UpdateCallbacks, cb)
}

func (inc *MOM2) EmitUpdate(val float64) {
	for _, cb := range inc.UpdateCallbacks {
		cb(val)
	}
}
