package bbgo

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
)

func RegisterModifier(s interface{}) {
	val := reflect.ValueOf(s)
	if val.Type().Kind() == util.Pointer {
		val = val.Elem()
	}
	var targetName string
	var currVal interface{}
	var mapping map[string]string
	// currently we only allow users to modify the first layer of fields
	RegisterCommand("/modify", "Modify config", func(reply interact.Reply) {
		reply.Message("Please choose the field name in config to modify:")
		mapping = make(map[string]string)
		dynamic.GetModifiableFields(val, func(tagName, name string) {
			mapping[tagName] = name
			reply.AddButton(tagName, tagName, tagName)
		})
	}).Next(func(target string, reply interact.Reply) error {
		targetName = mapping[target]
		field, ok := dynamic.GetModifiableField(val, targetName)
		if !ok {
			reply.Message(fmt.Sprintf("target %s is not modifiable", targetName))
			return fmt.Errorf("target %s is not modifiable", targetName)
		}
		currVal = field.Interface()
		if e, err := json.Marshal(currVal); err == nil {
			currVal = string(e)
		}
		reply.Message(fmt.Sprintf("Please enter the new value, current value: %v", currVal))
		return nil
	}).Next(func(value string, reply interact.Reply) {
		log.Infof("try to modify from %s to %s", currVal, value)
		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}
		field, ok := dynamic.GetModifiableField(val, targetName)
		if !ok {
			reply.Message(fmt.Sprintf("target %s is not modifiable", targetName))
			return
		}
		x := reflect.New(field.Type())
		xi := x.Interface()
		if err := json.Unmarshal([]byte(value), &xi); err != nil {
			reply.Message(fmt.Sprintf("fail to unmarshal the value: %s, err: %v", value, err))
			return
		}
		field.Set(x.Elem())
		newVal := field.Interface()
		if e, err := json.Marshal(value); err == nil {
			newVal = string(e)
		}
		reply.Message(fmt.Sprintf("update to %v successfully", newVal))
	})
}
