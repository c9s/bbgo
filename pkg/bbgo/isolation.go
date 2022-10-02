package bbgo

var currentIsolationContext *IsolationContext

func init() {
	currentIsolationContext = NewIsolationContext()
}

type IsolationContext struct {
	gracefulShutdown GracefulShutdown
}

func NewIsolationContext() *IsolationContext {
	return &IsolationContext{}
}
