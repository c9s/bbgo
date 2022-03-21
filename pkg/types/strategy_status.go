package types

// StrategyStatus define strategy status
type StrategyStatus string

const (
	StrategyStatusRunning StrategyStatus = "RUN"
	StrategyStatusStopped StrategyStatus = "STOP"
)
