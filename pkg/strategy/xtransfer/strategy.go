package xtransfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/bbgo"
	pred "github.com/c9s/bbgo/pkg/strategy/xtransfer/predicates"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "xtransfer"

type Strategy struct {
	*bbgo.Environment

	Interval   types.Duration   `json:"interval"`
	Predicates []pred.Predicate `json:"predicates"`
	logger     logrus.FieldLogger
}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	var ids []string = []string{ID}
	for _, p := range s.Predicates {
		ids = append(ids, p.String())
	}
	return strings.Join(ids, "-")
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	// wip
}

func (s *Strategy) Validate() error {
	if len(s.Predicates) == 0 {
		return fmt.Errorf("at least one predicate is required")
	}
	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = logrus.WithField("strategy", ID)

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	fmt.Println(s.InstanceID())
	return nil
}
