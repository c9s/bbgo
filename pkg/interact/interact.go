package interact

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type CustomInteraction interface {
	Commands(interact *Interact)
}

type Initializer interface {
	Initialize() error
}

type Messenger interface {
	TextMessageResponder
	CommandResponder
	Start(ctx context.Context)
}

// Interact implements the interaction between bot and message software.
type Interact struct {
	startTime time.Time

	// commands is the default public command map
	commands map[string]*Command

	// privateCommands is the private command map, need auth
	privateCommands map[string]*Command

	states     map[State]State
	statesFunc map[State]interface{}

	originState, currentState State

	customInteractions []CustomInteraction

	messenger Messenger
}

func New() *Interact {
	return &Interact{
		startTime:       time.Now(),
		commands:        make(map[string]*Command),
		privateCommands: make(map[string]*Command),
		originState:     StatePublic,
		currentState:    StatePublic,
		states:          make(map[State]State),
		statesFunc:      make(map[State]interface{}),
	}
}

func (it *Interact) SetOriginState(s State) {
	it.originState = s
}

func (it *Interact) AddCustomInteraction(custom CustomInteraction) {
	custom.Commands(it)
	it.customInteractions = append(it.customInteractions, custom)
}

func (it *Interact) PrivateCommand(command, desc string, f interface{}) *Command {
	cmd := NewCommand(command, desc, f)
	it.privateCommands[command] = cmd
	return cmd
}

func (it *Interact) Command(command string, desc string, f interface{}) *Command {
	cmd := NewCommand(command, desc, f)
	it.commands[command] = cmd
	return cmd
}

func (it *Interact) getNextState(currentState State) (nextState State, final bool) {
	var ok bool
	final = false
	nextState, ok = it.states[currentState]
	if ok {
		// check if it's the final state
		if _, hasTransition := it.statesFunc[nextState]; !hasTransition {
			final = true
		}

		return nextState, final
	}

	// state not found, return to the origin state
	return it.originState, final
}

func (it *Interact) SetState(s State) {
	log.Infof("[interact] transiting state from %s -> %s", it.currentState, s)
	it.currentState = s
}

func (it *Interact) handleResponse(text string, ctxObjects ...interface{}) error {
	// we only need response when executing a command
	switch it.currentState {
	case StatePublic, StateAuthenticated:
		return nil

	}

	args := parseCommand(text)

	f, ok := it.statesFunc[it.currentState]
	if !ok {
		return fmt.Errorf("state function of %s is not defined", it.currentState)
	}

	_, err := parseFuncArgsAndCall(f, args, ctxObjects...)
	if err != nil {
		return err
	}

	nextState, end := it.getNextState(it.currentState)
	if end {
		it.SetState(it.originState)
		return nil
	}

	it.SetState(nextState)
	return nil
}

func (it *Interact) getCommand(command string) (*Command, error) {
	switch it.currentState {
	case StateAuthenticated:
		if cmd, ok := it.privateCommands[command]; ok {
			return cmd, nil
		}

	case StatePublic:
		if _, ok := it.privateCommands[command]; ok {
			return nil, fmt.Errorf("private command can not be executed in the public mode")
		}

	}

	if cmd, ok := it.commands[command]; ok {
		return cmd, nil
	}

	return nil, fmt.Errorf("command %s not found", command)
}

func (it *Interact) runCommand(command string, args []string, ctxObjects ...interface{}) error {
	cmd, err := it.getCommand(command)
	if err != nil {
		return err
	}

	it.SetState(cmd.initState)
	if _, err := parseFuncArgsAndCall(cmd.F, args, ctxObjects...); err != nil {
		return err
	}

	// if we can successfully execute the command, then we can go to the next state.
	nextState, end := it.getNextState(it.currentState)
	if end {
		it.SetState(it.originState)
		return nil
	}

	it.SetState(nextState)
	return nil
}

func (it *Interact) SetMessenger(messenger Messenger) {
	// pass Responder function
	messenger.SetTextMessageResponder(func(message string, reply Reply, ctxObjects ...interface{}) error {
		return it.handleResponse(message, append(ctxObjects, reply)...)
	})
	it.messenger = messenger
}

// builtin initializes the built-in commands
func (it *Interact) builtin() error {
	it.Command("/uptime", "show bot uptime", func(reply Reply) error {
		uptime := time.Since(it.startTime)
		reply.Message(fmt.Sprintf("uptime %s", uptime))
		return nil
	})

	return nil
}

func (it *Interact) init() error {
	if err := it.builtin(); err != nil {
		return err
	}

	if err := it.registerCommands(it.commands); err != nil {
		return err
	}

	if err := it.registerCommands(it.privateCommands); err != nil {
		return err
	}

	return nil
}

func (it *Interact) registerCommands(commands map[string]*Command) error {
	for n, cmd := range commands {
		for s1, s2 := range cmd.states {
			if _, exist := it.states[s1]; exist {
				return fmt.Errorf("state %s already exists", s1)
			}

			it.states[s1] = s2
		}
		for s, f := range cmd.statesFunc {
			it.statesFunc[s] = f
		}

		// register commands to the service
		if it.messenger == nil {
			return fmt.Errorf("messenger is not set")
		}

		commandName := n
		it.messenger.AddCommand(cmd, func(message string, reply Reply, ctxObjects ...interface{}) error {
			args := parseCommand(message)
			return it.runCommand(commandName, args, append(ctxObjects, reply)...)
		})
	}
	return nil
}

func (it *Interact) Start(ctx context.Context) error {
	if err := it.init(); err != nil {
		return err
	}

	for _, custom := range it.customInteractions {
		log.Infof("checking %T custom interaction...", custom)
		if initializer, ok := custom.(Initializer); ok {
			log.Infof("initializing %T custom interaction...", custom)
			if err := initializer.Initialize(); err != nil {
				return err
			}
		}
	}

	// TODO: use go routine and context
	it.messenger.Start(ctx)
	return nil
}
