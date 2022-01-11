package bbgo

import (
	"errors"
	"go/scanner"
	"go/token"
	"reflect"
	"strconv"

	"gopkg.in/tucnak/telebot.v2"
)

// Interact implements the interaction between bot and message software.
type Interact struct {
	Commands map[string]func()
}

func (i *Interact) HandleTelegramMessage(msg *telebot.Message) {

}

func parseCommand(text string) (args []string, err error) {
	src := []byte(text)

	// Initialize the scanner.
	var errHandler scanner.ErrorHandler = func(pos token.Position, msg string) {
		err = errors.New(msg)
	}

	var s scanner.Scanner
	fset := token.NewFileSet()                      // positions are relative to fset
	file := fset.AddFile("", fset.Base(), len(src)) // register input "file"
	s.Init(file, src, errHandler, 0)

	// Repeated calls to Scan yield the token sequence found in the input.
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// we are not using the token type right now, but we will use them later
		args = append(args, lit)
	}

	return args, nil
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
