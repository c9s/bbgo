package interact

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/scanner"

	log "github.com/sirupsen/logrus"
)

type Reply interface {
	Message(message string)
	AddButton(text string)
	RemoveKeyboard()
}

type Responder func(reply Reply, response string) error

type CustomInteraction interface {
	Commands(interact *Interact)
}

type State string

const (
	StatePublic        State = "public"
	StateAuthenticated State = "authenticated"
)

type TextMessageResponder interface {
	SetTextMessageResponder(responder Responder)
}

type CommandResponder interface {
	AddCommand(command string, responder Responder)
}

type Messenger interface {
	TextMessageResponder
	CommandResponder
	Start()
}

// Interact implements the interaction between bot and message software.
type Interact struct {
	// commands is the default public command map
	commands map[string]*Command

	// privateCommands is the private command map, need auth
	privateCommands map[string]*Command

	states     map[State]State
	statesFunc map[State]interface{}

	originState, currentState State

	messenger Messenger
}

func New() *Interact {
	return &Interact{
		commands:     make(map[string]*Command),
		originState:  StatePublic,
		currentState: StatePublic,
		states:       make(map[State]State),
		statesFunc:   make(map[State]interface{}),
	}
}

func (i *Interact) SetOriginState(s State) {
	i.originState = s
}

func (i *Interact) AddCustomInteraction(custom CustomInteraction) {
	custom.Commands(i)
}

func (i *Interact) PrivateCommand(command string, f interface{}) *Command {
	cmd := NewCommand(command, f)
	i.privateCommands[command] = cmd
	return cmd
}

func (i *Interact) Command(command string, f interface{}) *Command {
	cmd := NewCommand(command, f)
	i.commands[command] = cmd
	return cmd
}

func (i *Interact) getNextState(currentState State) (nextState State, final bool) {
	var ok bool
	final = false
	nextState, ok = i.states[currentState]
	if ok {
		// check if it's the final state
		if _, hasTransition := i.statesFunc[nextState]; !hasTransition {
			final = true
		}

		return nextState, final
	}

	// state not found, return to the origin state
	return i.originState, final
}

func (i *Interact) setState(s State) {
	log.Infof("[interact]: transiting state from %s -> %s", i.currentState, s)
	i.currentState = s
}

func (i *Interact) handleResponse(text string, ctxObjects ...interface{}) error {
	args := parseCommand(text)

	f, ok := i.statesFunc[i.currentState]
	if !ok {
		return fmt.Errorf("state function of %s is not defined", i.currentState)
	}

	err := parseFuncArgsAndCall(f, args, ctxObjects...)
	if err != nil {
		return err
	}

	nextState, end := i.getNextState(i.currentState)
	if end {
		i.setState(i.originState)
		return nil
	}

	i.setState(nextState)
	return nil
}

func (i *Interact) getCommand(command string) (*Command, error) {
	switch i.currentState {
	case StateAuthenticated:
		if cmd, ok := i.privateCommands[command]; ok {
			return cmd, nil
		}

	case StatePublic:
		if _, ok := i.privateCommands[command]; ok {
			return nil, fmt.Errorf("private command can not be executed in the public mode")
		}

	}

	if cmd, ok := i.commands[command]; ok {
		return cmd, nil
	}

	return nil, fmt.Errorf("command %s not found", command)
}

func (i *Interact) runCommand(command string, args []string, ctxObjects ...interface{}) error {
	cmd, err := i.getCommand(command)
	if err != nil {
		return err
	}

	i.setState(cmd.initState)
	if err := parseFuncArgsAndCall(cmd.F, args, ctxObjects...); err != nil {
		return err
	}

	// if we can successfully execute the command, then we can go to the next state.
	nextState, end := i.getNextState(i.currentState)
	if end {
		i.setState(i.originState)
		return nil
	}

	i.setState(nextState)
	return nil
}

func (i *Interact) SetMessenger(messenger Messenger) {
	messenger.SetTextMessageResponder(func(reply Reply, response string) error {
		return i.handleResponse(response, reply)
	})
	i.messenger = messenger
}

// builtin initializes the built-in commands
func (i *Interact) builtin() error {
	i.Command("/auth", func(reply Reply) error {
		reply.Message("Enter your authentication code")
		return nil
	}).NamedNext(StateAuthenticated, func(reply Reply, code string) error {
		// check code
		reply.Message("Great! You're authenticated!")
		return nil
	})

	i.Command("/uptime", func(reply Reply) error {
		reply.Message("uptime")
		return nil
	})

	return nil
}

func (i *Interact) init() error {
	if err := i.builtin(); err != nil {
		return err
	}

	for n, cmd := range i.commands {
		for s1, s2 := range cmd.states {
			if _, exist := i.states[s1]; exist {
				return fmt.Errorf("state %s already exists", s1)
			}

			i.states[s1] = s2
		}
		for s, f := range cmd.statesFunc {
			i.statesFunc[s] = f
		}

		// register commands to the service
		if i.messenger == nil {
			return fmt.Errorf("messenger is not set")
		}

		commandName := n
		i.messenger.AddCommand(commandName, func(reply Reply, response string) error {
			args := parseCommand(response)
			return i.runCommand(commandName, args, reply)
		})
	}

	return nil
}

func (i *Interact) Start(ctx context.Context) error {
	if err := i.init(); err != nil {
		return err
	}

	// TODO: use go routine and context
	i.messenger.Start()
	return nil
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

func parseFuncArgsAndCall(f interface{}, args []string, objects ...interface{}) error {
	fv := reflect.ValueOf(f)
	ft := reflect.TypeOf(f)

	objectIndex := 0
	argIndex := 0

	var rArgs []reflect.Value
	for i := 0; i < ft.NumIn(); i++ {
		at := ft.In(i)

		switch k := at.Kind(); k {

		case reflect.Interface:
			found := false

			if objectIndex >= len(objects) {
				return fmt.Errorf("found interface type %s, but object args are empty", at)
			}

			for oi := objectIndex; oi < len(objects); oi++ {
				obj := objects[oi]
				objT := reflect.TypeOf(obj)
				objV := reflect.ValueOf(obj)

				fmt.Println(
					at.PkgPath(),
					at.Name(),
					objT, "implements", at, "=", objT.Implements(at),
				)

				if objT.Implements(at) {
					found = true
					rArgs = append(rArgs, objV)
					objectIndex = oi + 1
					break
				}
			}
			if !found {
				return fmt.Errorf("can not find object implements %s", at)
			}

		case reflect.String:
			av := reflect.ValueOf(args[argIndex])
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Bool:
			bv, err := strconv.ParseBool(args[argIndex])
			if err != nil {
				return err
			}
			av := reflect.ValueOf(bv)
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Int64:
			nf, err := strconv.ParseInt(args[argIndex], 10, 64)
			if err != nil {
				return err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)
			argIndex++

		case reflect.Float64:
			nf, err := strconv.ParseFloat(args[argIndex], 64)
			if err != nil {
				return err
			}

			av := reflect.ValueOf(nf)
			rArgs = append(rArgs, av)
			argIndex++
		}
	}

	out := fv.Call(rArgs)
	if ft.NumOut() == 0 {
		return nil
	}

	// try to get the error object from the return value
	for i := 0; i < ft.NumOut(); i++ {
		outType := ft.Out(i)
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
