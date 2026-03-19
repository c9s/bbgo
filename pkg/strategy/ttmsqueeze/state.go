package ttmsqueeze

// State represents the current state of the strategy
type State int

const (
	// StateIdle indicates no position, waiting for entry signals
	StateIdle State = iota

	// StateLong indicates the strategy has a positive position
	// Can increase position or do staged exits while in this state
	StateLong

	// StateExiting indicates hard exit in progress, closing entire position
	// No new entries allowed until position is fully closed
	StateExiting
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateLong:
		return "Long"
	case StateExiting:
		return "Exiting"
	default:
		return "Unknown"
	}
}
