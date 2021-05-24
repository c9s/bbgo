package xbalance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

const ID = "xbalance"

const stateKey = "state-v1"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Asset                  string           `json:"asset"`
	DailyNumberOfTransfers int              `json:"dailyNumberOfTransfers,omitempty"`
	DailyAmountOfTransfers fixedpoint.Value `json:"dailyAmountOfTransfers,omitempty"`
	Since                  int64            `json:"since"`
}

func (s *State) IsOver24Hours() bool {
	return time.Now().Sub(time.Unix(s.Since, 0)) >= 24*time.Hour
}

func (s *State) PlainText() string {
	return util.Render(`{{ .Asset }} transfer stats:
daily number of transfers: {{ .DailyNumberOfTransfers }}
daily amount of transfers {{ .DailyAmountOfTransfers.Float64 }}`, s)
}

func (s *State) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title: s.Asset + " Transfer States",
		Fields: []slack.AttachmentField{
			{Title: "Total Number of Transfers", Value: fmt.Sprintf("%d", s.DailyNumberOfTransfers), Short: true},
			{Title: "Total Amount of Transfers", Value: util.FormatFloat(s.DailyAmountOfTransfers.Float64(), 4), Short: true},
		},
		Footer: util.Render("Since {{ . }}", time.Unix(s.Since, 0).Format(time.RFC822)),
	}
}

func (s *State) Reset() {
	var beginningOfTheDay = Bod(time.Now())
	*s = State{
		DailyNumberOfTransfers: 0,
		DailyAmountOfTransfers: 0,
		Since:                  beginningOfTheDay.Unix(),
	}
}

type WithdrawalRequest struct {
	FromSession string           `json:"fromSession"`
	ToSession   string           `json:"toSession"`
	Asset       string           `json:"asset"`
	Amount      fixedpoint.Value `json:"amount"`
}

func (r *WithdrawalRequest) String() string {
	return fmt.Sprintf("WITHDRAWAL REQUEST: sending %s %s from %s -> %s",
		util.FormatFloat(r.Amount.Float64(), 4),
		r.Asset,
		r.FromSession,
		r.ToSession,
	)
}

func (r *WithdrawalRequest) PlainText() string {
	return fmt.Sprintf("Withdrawal request: sending %s %s from %s -> %s",
		util.FormatFloat(r.Amount.Float64(), 4),
		r.Asset,
		r.FromSession,
		r.ToSession,
	)
}

func (r *WithdrawalRequest) SlackAttachment() slack.Attachment {
	var color = "#DC143C"
	title := util.Render(`Withdrawal Request {{ .Asset }}`, r)
	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title: title,
		Color: color,
		Fields: []slack.AttachmentField{
			{Title: "Asset", Value: r.Asset, Short: true},
			{Title: "Amount", Value: util.FormatFloat(r.Amount.Float64(), 4), Short: true},
			{Title: "From", Value: r.FromSession},
			{Title: "To", Value: r.ToSession},
		},
		Footer: util.Render("Time {{ . }}", time.Now().Format(time.RFC822)),
		// FooterIcon: "",
	}
}

type Strategy struct {
	Notifiability *bbgo.Notifiability
	*bbgo.Graceful
	*bbgo.Persistence

	Interval types.Duration `json:"interval"`

	Addresses map[string]string `json:"addresses"`

	MaxDailyNumberOfTransfer int              `json:"maxDailyNumberOfTransfer"`
	MaxDailyAmountOfTransfer fixedpoint.Value `json:"maxDailyAmountOfTransfer"`

	CheckOnStart bool `json:"checkOnStart"`

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

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) checkBalance(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {

	s.Notifiability.Notify("📝 Checking %s low balance level exchange session...", s.Asset)

	lowLevelSession, lowLevelBalance, err := s.findLowBalanceLevelSession(sessions)
	if err != nil {
		log.WithError(err).Errorf("can not find low balance level session")
		return
	}

	if lowLevelSession == nil {
		s.Notifiability.Notify("✅ All %s balances are looking good", s.Asset)
		return
	}

	s.Notifiability.Notify("⚠️ Found low level %s balance from session %s: %s", s.Asset, lowLevelSession.Name, lowLevelBalance.String())

	requiredAmount := s.Middle - lowLevelBalance.Available

	s.Notifiability.Notify("Need %f %s to satisfy the middle balance level %f", requiredAmount.Float64(), s.Asset, s.Middle.Float64())

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
		s.Notifiability.Notify("The withdrawal function exchange session %s is not enabled", fromSession.Name)
		log.Errorf("The withdrawal function of exchange session %s is not enabled", fromSession.Name)
		return
	}

	toAddress, ok := s.Addresses[lowLevelSession.Name]
	if !ok {
		log.Errorf("%s address of session %s not found", s.Asset, lowLevelSession.Name)
		return
	}

	if s.state != nil {
		if s.MaxDailyNumberOfTransfer > 0 {
			if s.state.DailyNumberOfTransfers >= s.MaxDailyNumberOfTransfer {
				s.Notifiability.Notify("⚠️ Exceeded %s max daily number of transfers %d (current %d), skipping transfer...",
					s.Asset,
					s.MaxDailyNumberOfTransfer,
					s.state.DailyNumberOfTransfers)
				return
			}
		}

		if s.MaxDailyAmountOfTransfer > 0 {
			if s.state.DailyAmountOfTransfers >= s.MaxDailyAmountOfTransfer {
				s.Notifiability.Notify("⚠️ Exceeded %s max daily amount of transfers %f (current %f), skipping transfer...",
					s.Asset,
					s.MaxDailyAmountOfTransfer.Float64(),
					s.state.DailyAmountOfTransfers.Float64())
				return
			}
		}
	}

	s.Notifiability.Notify(&WithdrawalRequest{
		FromSession: fromSession.Name,
		ToSession:   lowLevelSession.Name,
		Asset:       s.Asset,
		Amount:      requiredAmount,
	})

	if err := withdrawalService.Withdrawal(ctx, s.Asset, requiredAmount, toAddress); err != nil {
		log.WithError(err).Errorf("withdrawal failed")
		s.Notifiability.Notify("withdrawal request failed, error: %v", err)
		return
	}

	s.Notifiability.Notify("%s withdrawal request sent", s.Asset)

	if s.state != nil {
		if s.state.IsOver24Hours() {
			s.state.Reset()
		}

		s.state.DailyNumberOfTransfers += 1
		s.state.DailyAmountOfTransfers += requiredAmount
		s.SaveState()
	}
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

func Bod(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func (s *Strategy) SaveState() {
	if err := s.Persistence.Save(s.state, ID, s.Asset, stateKey); err != nil {
		log.WithError(err).Errorf("can not save state: %+v", s.state)
	} else {
		log.Infof("%s %s state is saved: %+v", ID, s.Asset, s.state)
		s.Notifiability.Notify("%s %s state is saved", ID, s.Asset, s.state)
	}
}

func (s *Strategy) newDefaultState() *State {
	return &State{
		Asset:                  s.Asset,
		DailyNumberOfTransfers: 0,
		DailyAmountOfTransfers: 0,
	}
}

func (s *Strategy) LoadState() error {
	var state State
	if err := s.Persistence.Load(&state, ID, s.Asset, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = s.newDefaultState()
		s.state.Reset()
	} else {
		// we loaded it successfully
		s.state = &state

		// update Asset name for legacy caches
		s.state.Asset = s.Asset

		log.Infof("%s %s state is restored: %+v", ID, s.Asset, s.state)
		s.Notifiability.Notify("%s %s state is restored", ID, s.Asset, s.state)
	}

	return nil
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.Interval == 0 {
		return errors.New("interval can not be zero")
	}

	if err := s.LoadState(); err != nil {
		return err
	}

	s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		s.SaveState()
	})

	if s.CheckOnStart {
		s.checkBalance(ctx, sessions)
	}

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
