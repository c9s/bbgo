package interact

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/scanner"

	"gopkg.in/tucnak/telebot.v2"
)

type Command struct {
	// Name is the command name
	Name string

	// StateF is the command handler function
	F interface{}

	stateID    int
	states     map[string]string
	statesFunc map[string]interface{}
	initState  string
}

func NewCommand(name string, f interface{}) *Command {
	c := &Command{
		Name:       name,
		F:          f,
		states:     make(map[string]string),
		statesFunc: make(map[string]interface{}),
		initState:  name + "_" + strconv.Itoa(0),
	}
	return c.Next(f)
}

func (c *Command) Next(f interface{}) *Command {
	curState := c.Name + "_" + strconv.Itoa(c.stateID)
	c.stateID++
	nextState := c.Name + "_" + strconv.Itoa(c.stateID)

	c.states[curState] = nextState
	c.statesFunc[curState] = f
	return c
}

// Interact implements the interaction between bot and message software.
type Interact struct {
	commands map[string]*Command

	states     map[string]string
	statesFunc map[string]interface{}
	curState   string
}

func New() *Interact {
	return &Interact{
		commands:   make(map[string]*Command),
		states:     make(map[string]string),
		statesFunc: make(map[string]interface{}),
	}
}

func (i *Interact) Command(command string, f interface{}) *Command {
	cmd := NewCommand(command, f)
	i.commands[command] = cmd
	return cmd
}

func (i *Interact) getNextState(currentState string) (nextState string, end bool) {
	var ok bool
	nextState, ok = i.states[currentState]
	if ok {
		end = false
		return nextState, end
	}

	end = true
	return nextState, end
}

func (i *Interact) handleResponse(text string) error {
	args := parseCommand(text)

	f, ok := i.statesFunc[i.curState]
	if !ok {
		return fmt.Errorf("state function of %s is not defined", i.curState)
	}

	err := parseFuncArgsAndCall(f, args)
	if err != nil {
		return err
	}

	nextState, end := i.getNextState(i.curState)
	if end {
		return nil
	}

	i.curState = nextState
	return nil
}

func (i *Interact) runCommand(command string, args ...string) error {
	cmd, ok := i.commands[command]
	if !ok {
		return fmt.Errorf("command %s not found", command)
	}

	i.curState = cmd.initState
	err := parseFuncArgsAndCall(cmd.F, args)
	if err != nil {
		return err
	}

	// if we can successfully execute the command, then we can go to the next state.
	nextState, end := i.getNextState(i.curState)
	if end {
		return nil
	}

	i.curState = nextState
	return nil
}

func (i *Interact) init() error {
	for n, cmd := range i.commands {
		_ = n
		for s1, s2 := range cmd.states {
			if _, exist := i.states[s1]; exist {
				return fmt.Errorf("state %s already exists", s1)
			}

			i.states[s1] = s2
		}
		for s, f := range cmd.statesFunc {
			i.statesFunc[s] = f
		}
	}

	return nil
}

func (i *Interact) HandleTelegramMessage(msg *telebot.Message) {
	// For registered commands, will contain the string payload
	// msg.Payload
	// msg.Text
	args := parseCommand(msg.Text)
	_ = args
}

func parseCommand(src string) (args []string) {
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
