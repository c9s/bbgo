package bbgo

import (
	"reflect"
	"strconv"
	"strings"
	"text/scanner"

	"gopkg.in/tucnak/telebot.v2"
)

// Interact implements the interaction between bot and message software.
type Interact struct {
	Commands map[string]func()
}

func (i *Interact) HandleTelegramMessage(msg *telebot.Message) {

}

func parseCommand(src string) (args []string) {
	var s scanner.Scanner
	s.Init(strings.NewReader(src))
	s.Filename = "command"
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		text := s.TokenText()
		if text[0] == '"' && text[len(text) - 1] == '"' {
			text, _ = strconv.Unquote(text)
		}
		args = append(args,text)
	}

	return args
}

func parseFuncArgsAndCall(f interface{}, args []string) error {
	fv := reflect.ValueOf(f)
	ft := reflect.TypeOf(f)

	var rArgs []reflect.Value
	for i := 0; i < ft.NumIn(); i++ {
		at := ft.In(i)

		switch k := at.Kind(); k {

		case reflect.String:
			av := reflect.ValueOf(args[i])
			rArgs = append(rArgs, av)

		case reflect.Bool:
			bv, err := strconv.ParseBool(args[i])
			if err != nil {
				return err
			}
			av := reflect.ValueOf(bv)
			rArgs = append(rArgs, av)

		case reflect.Int64:
			nf, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)

		case reflect.Float64:
			nf, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)
		}
	}

	out := fv.Call(rArgs)
	if ft.NumOut() > 0 {
		outType := ft.Out(0)
		switch outType.Kind() {
		case reflect.Interface:
			o := out[0].Interface()
			switch ov := o.(type) {
			case error:
				return ov

			}

		}
	}
	return nil
}
