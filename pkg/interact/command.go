package interact

import "strconv"

// Command is a domain specific language syntax helper
// It's used for helping developer define the state and transition function
type Command struct {
	// Name is the command name
	Name string

	// StateF is the command handler function
	F interface{}

	stateID              int
	states               map[State]State
	statesFunc           map[State]interface{}
	initState, lastState State
}

func NewCommand(name string, f interface{}) *Command {
	c := &Command{
		Name:       name,
		F:          f,
		states:     make(map[State]State),
		statesFunc: make(map[State]interface{}),
		initState:  State(name + "_" + strconv.Itoa(0)),
	}
	return c.Next(f)
}

// Transit defines the state transition that is not related to the last defined state.
func (c *Command) Transit(state1, state2 State, f interface{}) *Command {
	c.states[state1] = state2
	c.statesFunc[state1] = f
	return c
}

func (c *Command) NamedNext(n string, f interface{}) *Command {
	var curState State
	if c.lastState == "" {
		curState = State(c.Name + "_" + strconv.Itoa(c.stateID))
	} else {
		curState = c.lastState
	}

	nextState := State(n)
	c.states[curState] = nextState
	c.statesFunc[curState] = f
	c.lastState = nextState
	return c
}

// Next defines the next state with the transition function from the last defined state.
func (c *Command) Next(f interface{}) *Command {
	var curState State
	if c.lastState == "" {
		curState = State(c.Name + "_" + strconv.Itoa(c.stateID))
	} else {
		curState = c.lastState
	}

	// generate the next state by the stateID
	c.stateID++
	nextState := State(c.Name + "_" + strconv.Itoa(c.stateID))

	c.states[curState] = nextState
	c.statesFunc[curState] = f
	c.lastState = nextState
	return c
}
