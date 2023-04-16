package interact

import (
	"context"
	"fmt"
	"sync"
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

type Session interface {
	ID() string
	SetOriginState(state State)
	GetOriginState() State
	SetState(state State)
	GetState() State
	IsAuthorized() bool
	SetAuthorized()
	SetAuthorizing(b bool)
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

	customInteractions []CustomInteraction

	messengers []Messenger

	mu sync.Mutex
}

func New() *Interact {
	return &Interact{
		startTime:       time.Now(),
		commands:        make(map[string]*Command),
		privateCommands: make(map[string]*Command),
		states:          make(map[State]State),
		statesFunc:      make(map[State]interface{}),
	}
}

func (it *Interact) AddCustomInteraction(custom CustomInteraction) {
	custom.Commands(it)

	it.mu.Lock()
	it.customInteractions = append(it.customInteractions, custom)
	it.mu.Unlock()
}

func (it *Interact) PrivateCommand(command, desc string, f interface{}) *Command {
	cmd := NewCommand(command, desc, f)
	it.mu.Lock()
	it.privateCommands[command] = cmd
	it.mu.Unlock()
	return cmd
}

func (it *Interact) Command(command string, desc string, f interface{}) *Command {
	cmd := NewCommand(command, desc, f)
	it.mu.Lock()
	it.commands[command] = cmd
	it.mu.Unlock()
	return cmd
}

func (it *Interact) getNextState(session Session, currentState State) (nextState State, final bool) {
	var ok bool
	final = false

	it.mu.Lock()
	nextState, ok = it.states[currentState]
	it.mu.Unlock()

	if ok {
		// check if it's the final state
		if _, hasTransition := it.statesFunc[nextState]; !hasTransition {
			final = true
		}

		return nextState, final
	}

	// state not found, return to the origin state
	return session.GetOriginState(), final
}

func (it *Interact) handleResponse(session Session, text string, ctxObjects ...interface{}) error {
	// We only need response when executing a command
	switch session.GetState() {
	case StatePublic, StateAuthenticated:
		return nil

	}

	args := parseCommand(text)

	state := session.GetState()
	f, ok := it.statesFunc[state]
	if !ok {
		return fmt.Errorf("state function of %s is not defined", state)
	}

	ctxObjects = append(ctxObjects, session)
	_, err := ParseFuncArgsAndCall(f, args, ctxObjects...)
	if err != nil {
		return err
	}

	nextState, end := it.getNextState(session, state)
	if end {
		session.SetState(session.GetOriginState())
		return nil
	}

	session.SetState(nextState)
	return nil
}

func (it *Interact) getCommand(session Session, command string) (*Command, error) {
	it.mu.Lock()
	defer it.mu.Unlock()

	if session.IsAuthorized() {
		if cmd, ok := it.privateCommands[command]; ok {
			return cmd, nil
		}
	} else {
		if _, ok := it.privateCommands[command]; ok {
			return nil, fmt.Errorf("private command can not be executed in the public mode, type /auth to get authorized")
		}
	}

	// find any public command
	if cmd, ok := it.commands[command]; ok {
		return cmd, nil
	}

	return nil, fmt.Errorf("command %s not found", command)
}

func (it *Interact) runCommand(session Session, command string, args []string, ctxObjects ...interface{}) error {
	cmd, err := it.getCommand(session, command)
	if err != nil {
		return err
	}

	ctxObjects = append(ctxObjects, session)
	session.SetState(cmd.initState)
	if _, err := ParseFuncArgsAndCall(cmd.F, args, ctxObjects...); err != nil {
		return err
	}

	// if we can successfully execute the command, then we can go to the next state.
	state := session.GetState()
	nextState, end := it.getNextState(session, state)
	if end {
		session.SetState(session.GetOriginState())
		return nil
	}

	session.SetState(nextState)
	return nil
}

func (it *Interact) AddMessenger(messenger Messenger) {
	// pass Responder function
	messenger.SetTextMessageResponder(func(session Session, message string, reply Reply, ctxObjects ...interface{}) error {
		return it.handleResponse(session, message, append(ctxObjects, reply)...)
	})
	it.messengers = append(it.messengers, messenger)
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
		if len(it.messengers) == 0 {
			return fmt.Errorf("messenger is not set")
		}

		// commandName is used in the closure, we need to copy the variable
		commandName := n
		for _, messenger := range it.messengers {
			messenger.AddCommand(cmd, func(session Session, message string, reply Reply, ctxObjects ...interface{}) error {
				args := parseCommand(message)
				return it.runCommand(session, commandName, args, append(ctxObjects, reply)...)
			})
		}
	}
	return nil
}

func (it *Interact) Start(ctx context.Context) error {
	if len(it.messengers) == 0 {
		log.Warn("messenger is not set, skip initializing")
		return nil
	}

	if err := it.init(); err != nil {
		return err
	}

	for _, custom := range it.customInteractions {
		log.Debugf("checking %T custom interaction...", custom)
		if initializer, ok := custom.(Initializer); ok {
			log.Debugf("initializing %T custom interaction...", custom)
			if err := initializer.Initialize(); err != nil {
				return err
			}
		}
	}

	// TODO: use go routine and context
	for _, m := range it.messengers {
		go m.Start(ctx)
	}
	return nil
}
