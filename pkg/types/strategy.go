package types

// Strategy method calls:
// -> Defaults()     (optional method)
//
//	setup default static values from constants
//
// -> Initialize()   (optional method)
//
//	initialize dynamic runtime objects
//
// -> Subscribe()
//
//	register the subscriptions
//
// -> Validate()     (optional method)
// -> Run()          (optional method)
// -> Shutdown(shutdownCtx context.Context, wg *sync.WaitGroup)
type StrategyID interface {
	ID() string
}
