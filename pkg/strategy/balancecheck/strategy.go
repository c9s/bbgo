package balancecheck

import (
	"context"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/risk/riskcontrol"
)

const ID = "balancecheck"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	*riskcontrol.BalanceCheck
	CronExpression string `json:"cronExpression"`

	cron *cron.Cron
}

func (s *Strategy) Defaults() error {
	return nil
}

func (s *Strategy) Initialize() error {
	s.BalanceCheck = &riskcontrol.BalanceCheck{
		ExpectedBalances:       s.ExpectedBalances,
		BalanceCheckTorlerance: s.BalanceCheckTorlerance,
	}
	return nil
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if err := s.BalanceCheck.Validate(); err != nil {
		return err
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.cron = cron.New()
	s.cron.AddFunc(s.CronExpression, func() {
		balances := session.GetAccount().Balances()
		if err := s.Check(balances); err != nil {
			log.WithError(err).Error("balance check failed")
		}
	})
	s.cron.Start()
	return nil
}
