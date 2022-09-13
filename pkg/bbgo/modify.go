package bbgo

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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
		for i := 0; i < val.Type().NumField(); i++ {
			t := val.Type().Field(i)
			if !t.IsExported() {
				continue
			}
			modifiable := t.Tag.Get("modifiable")
			if modifiable != "true" {
				continue
			}
			jsonTag := t.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			name := strings.Split(jsonTag, ",")[0]
			mapping[name] = t.Name
			reply.AddButton(name, name, name)
		}
	}).Next(func(target string, reply interact.Reply) {
		targetName = mapping[target]
		field := val.FieldByName(targetName)
		currVal = field.Interface()
		if e, err := json.Marshal(currVal); err == nil {
			currVal = string(e)
		}
		reply.Message(fmt.Sprintf("Please enter the new value, current value: %v", currVal))
	}).Next(func(value string, reply interact.Reply) {
		log.Infof("%s", value)
		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}
		field := val.FieldByName(targetName)
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

	RegisterCommand("/save", "Save config", func(reply interact.Reply) {
		reply.Message("")
	})
}
