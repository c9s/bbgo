package types

// StrategyStatus define strategy status
type StrategyStatus string

const (
	StrategyStatusRunning StrategyStatus = "RUNNING"
	StrategyStatusStopped StrategyStatus = "STOPPED"
)
