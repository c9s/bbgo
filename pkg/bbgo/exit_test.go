package bbgo

import "testing"

func TestExitMethod(t *testing.T) {
	em := &ExitMethod{}
	em.Subscribe(&ExchangeSession{})
}
