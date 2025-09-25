package predicates

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type marginLevelPredicate struct {
	MinMarginLeval    fixedpoint.Value `json:"minMarginLevel"`
	TargetMarginLeval fixedpoint.Value `json:"targetMarginLevel"`
}

func (p *marginLevelPredicate) Eval(sessions map[string]*bbgo.ExchangeSession) bool {
	return false
}

func (p *marginLevelPredicate) Do(sessions map[string]*bbgo.ExchangeSession) error {
	return nil
}

func (p *marginLevelPredicate) String() string {
	return fmt.Sprintf("marginLevel(%s, %s)", p.MinMarginLeval, p.TargetMarginLeval)
}
