package interact

import (
	"reflect"
	"strconv"
	"strings"
	"text/scanner"

	"github.com/mattn/go-shellwords"
	log "github.com/sirupsen/logrus"
)

func ParseFuncArgsAndCall(f interface{}, args []string, objects ...interface{}) (State, error) {
	fv := reflect.ValueOf(f)
	ft := reflect.TypeOf(f)
	argIndex := 0

	var rArgs []reflect.Value
	for i := 0; i < ft.NumIn(); i++ {
		at := ft.In(i)

		// get the kind of argument
		switch k := at.Kind(); k {

		case reflect.Interface:
			found := false
			for oi := 0; oi < len(objects); oi++ {
				obj := objects[oi]
				objT := reflect.TypeOf(obj)
				objV := reflect.ValueOf(obj)

				log.Debugln(
					at.PkgPath(),
					at.Name(),
					objT, "implements", at, "=", objT.Implements(at),
				)

				if objT.Implements(at) {
					found = true
					rArgs = append(rArgs, objV)
					break
				}
			}

			if !found {
				v := reflect.Zero(at)
				rArgs = append(rArgs, v)
			}

		case reflect.String:
			av := reflect.ValueOf(args[argIndex])
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Bool:
			bv, err := strconv.ParseBool(args[argIndex])
			if err != nil {
				return "", err
			}
			av := reflect.ValueOf(bv)
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Int64:
			nf, err := strconv.ParseInt(args[argIndex], 10, 64)
			if err != nil {
				return "", err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Float64:
			nf, err := strconv.ParseFloat(args[argIndex], 64)
			if err != nil {
				return "", err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)
			argIndex++
		}
	}

	out := fv.Call(rArgs)
	if ft.NumOut() == 0 {
		return "", nil
	}

	// try to get the error object from the return value
	var err error
	var state State
	for i := 0; i < ft.NumOut(); i++ {
		outType := ft.Out(i)
		switch outType.Kind() {
		case reflect.String:
			if outType.Name() == "State" {
				state = State(out[i].String())
			}

		case reflect.Interface:
			o := out[i].Interface()
			switch ov := o.(type) {
			case error:
				err = ov

			}
		}
	}
	return state, err
}

func parseCommand(src string) (args []string) {
	var err error
	args, err = shellwords.Parse(src)
	if err == nil {
		return args
	}

	// fallback to go text/scanner
	var s scanner.Scanner
	s.Init(strings.NewReader(src))
	s.Filename = "command"
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		text := s.TokenText()
		if text[0] == '"' && text[len(text)-1] == '"' {
			text, _ = strconv.Unquote(text)
		}
		args = append(args, text)
	}

	return args
}
