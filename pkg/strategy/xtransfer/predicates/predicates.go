package predicates

import (
	"github.com/c9s/bbgo/pkg/bbgo"
)

type Predicate struct {
	MarginLevelPredicate *marginLevelPredicate `json:"marginLevelPredicate,omitempty"`
}

func (p *Predicate) Eval(sessions map[string]*bbgo.ExchangeSession) bool {
	if p.MarginLevelPredicate != nil {
		return p.MarginLevelPredicate.Eval(sessions)
	}
	return false
}

func (p *Predicate) Do(sessions map[string]*bbgo.ExchangeSession) error {
	if p.MarginLevelPredicate != nil {
		return p.MarginLevelPredicate.Do(sessions)
	}
	return nil
}

func (p *Predicate) String() string {
	if p.MarginLevelPredicate != nil {
		return p.MarginLevelPredicate.String()
	}
	return "unknown"
}
