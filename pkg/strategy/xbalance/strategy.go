package xbalance

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const ID = "xbalance"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	DailyNumberOfTransfers fixedpoint.Value `json:"dailyNumberOfTransfers,omitempty"`
	DailyAmountOfTransfers fixedpoint.Value `json:"dailyAmountOfTransfers,omitempty"`
	Since                  int64            `json:"since"`
}

type Strategy struct {
	Notifiability *bbgo.Notifiability

	Interval types.Duration `json:"interval"`

	Addresses map[string]string `json:"addresses"`

	MaxDailyNumberOfTransfer int `json:"maxDailyNumberOfTransfer"`
	MaxDailyAmountOfTransfer int `json:"maxDailyAmountOfTransfer"`

	Asset string `json:"asset"`

	// Low is the low balance level for triggering transfer
	Low fixedpoint.Value `json:"low"`

	// Middle is the middle balance level used for re-fill asset
	Middle fixedpoint.Value `json:"middle"`

	state *State
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) { }

func (s *Strategy) checkBalance(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	s.Notifiability.Notify("Checking %s low balance level exchange session...", s.Asset)

	lowLevelSession, lowLevelBalance, err := s.findLowBalanceLevelSession(sessions)
	if err != nil {
		log.WithError(err).Errorf("can not find low balance level session")
		return
	}

	if lowLevelSession == nil {
		s.Notifiability.Notify("All %s balances are looking good", s.Asset)
		return
	}

	s.Notifiability.Notify("Found %s low balance level %s", s.Asset, lowLevelBalance.String())

	requiredAmount := s.Middle - lowLevelBalance.Available

	s.Notifiability.Notify("Require %f %s to satisfy the middle balance level %f", requiredAmount.Float64(), s.Asset, s.Middle.Float64())

	fromSession, _, err := s.findHighestBalanceLevelSession(sessions, requiredAmount)
	if err != nil || fromSession == nil {
		log.WithError(err).Errorf("can not find session with enough balance")
		return
	}

	withdrawalService, ok := fromSession.Exchange.(types.ExchangeWithdrawalService)
	if !ok {
		log.Errorf("exchange %s does not implement withdrawal service, we can not withdrawal", fromSession.ExchangeName)
		return
	}

	if !fromSession.Withdrawal {
		s.Notifiability.Notify("the withdrawal function exchange session %s is not enabled", fromSession.Name)
		log.Errorf("The withdrawal function of exchange session %s is not enabled", fromSession.Name)
		return
	}

	toAddress, ok := s.Addresses[lowLevelSession.Name]
	if !ok {
		log.Errorf("%s address of session %s not found", s.Asset, lowLevelSession.Name)
		return
	}

	s.Notifiability.Notify("Sending %f %s withdrawal request from session %s to session %s...", requiredAmount.Float64(), s.Asset, fromSession.Name, lowLevelSession.Name)
	if err := withdrawalService.Withdrawal(ctx, s.Asset, requiredAmount, toAddress); err != nil {
		log.WithError(err).Errorf("withdrawal failed")
		s.Notifiability.Notify("withdrawal request failed, error: %v", err)
		return
	}

	s.Notifiability.Notify("%s Withdrawal request sent", s.Asset)
}

func (s *Strategy) findHighestBalanceLevelSession(sessions map[string]*bbgo.ExchangeSession, requiredAmount fixedpoint.Value) (*bbgo.ExchangeSession, types.Balance, error) {
	var balance types.Balance
	var maxBalanceLevel fixedpoint.Value = 0
	var maxBalanceSession *bbgo.ExchangeSession = nil
	for sessionID := range s.Addresses {
		session, ok := sessions[sessionID]
		if !ok {
			return nil, balance, fmt.Errorf("session %s does not exist", sessionID)
		}

		if b, ok := session.Account.Balance(s.Asset); ok {
			if b.Available-requiredAmount > s.Low && b.Available > maxBalanceLevel {
				maxBalanceLevel = b.Available
				maxBalanceSession = session
				balance = b
			}
		}
	}

	return maxBalanceSession, balance, nil
}

func (s *Strategy) findLowBalanceLevelSession(sessions map[string]*bbgo.ExchangeSession) (*bbgo.ExchangeSession, types.Balance, error) {
	var balance types.Balance
	for sessionID := range s.Addresses {
		session, ok := sessions[sessionID]
		if !ok {
			return nil, balance, fmt.Errorf("session %s does not exist", sessionID)
		}

		balance, ok = session.Account.Balance(s.Asset)
		if ok {
			if balance.Available <= s.Low {
				return session, balance, nil
			}
		}
	}

	return nil, balance, nil
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.Interval == 0 {
		return errors.New("interval can not be zero")
	}

	s.checkBalance(ctx, sessions)

	go func() {
		ticker := time.NewTimer(durationJitter(s.Interval.Duration(), 1000))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.checkBalance(ctx, sessions)
			}
		}
	}()

	return nil
}

func durationJitter(d time.Duration, jitterInMilliseconds int) time.Duration {
	n := rand.Intn(jitterInMilliseconds)
	return d + time.Duration(n)*time.Millisecond
}
