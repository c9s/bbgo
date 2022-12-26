package xbalance

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/templateutil"
)

const ID = "xbalance"

const stateKey = "state-v1"

var priceFixer = fixedpoint.NewFromFloat(0.99)

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
	return time.Since(time.Unix(s.Since, 0)) >= 24*time.Hour
}

func (s *State) PlainText() string {
	return templateutil.Render(`{{ .Asset }} transfer stats:
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
			{Title: "Total Amount of Transfers", Value: s.DailyAmountOfTransfers.String(), Short: true},
		},
		Footer: templateutil.Render("Since {{ . }}", time.Unix(s.Since, 0).Format(time.RFC822)),
	}
}

func (s *State) Reset() {
	var beginningOfTheDay = types.BeginningOfTheDay(time.Now().Local())
	*s = State{
		DailyNumberOfTransfers: 0,
		DailyAmountOfTransfers: fixedpoint.Zero,
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
		r.Amount.FormatString(4),
		r.Asset,
		r.FromSession,
		r.ToSession,
	)
}

func (r *WithdrawalRequest) PlainText() string {
	return fmt.Sprintf("Withdraw request: sending %s %s from %s -> %s",
		r.Amount.FormatString(4),
		r.Asset,
		r.FromSession,
		r.ToSession,
	)
}

func (r *WithdrawalRequest) SlackAttachment() slack.Attachment {
	var color = "#DC143C"
	title := templateutil.Render(`Withdraw Request {{ .Asset }}`, r)
	return slack.Attachment{
		// Pretext:       "",
		// Text:  text,
		Title: title,
		Color: color,
		Fields: []slack.AttachmentField{
			{Title: "Asset", Value: r.Asset, Short: true},
			{Title: "Amount", Value: r.Amount.FormatString(4), Short: true},
			{Title: "From", Value: r.FromSession},
			{Title: "To", Value: r.ToSession},
		},
		Footer: templateutil.Render("Time {{ . }}", time.Now().Format(time.RFC822)),
		// FooterIcon: "",
	}
}

type Address struct {
	Address    string           `json:"address"`
	AddressTag string           `json:"addressTag"`
	Network    string           `json:"network"`
	ForeignFee fixedpoint.Value `json:"foreignFee"`
}

func (a *Address) UnmarshalJSON(body []byte) error {
	var arg interface{}
	err := json.Unmarshal(body, &arg)
	if err != nil {
		return err
	}

	switch argT := arg.(type) {
	case string:
		a.Address = argT
		return nil
	}

	type addressTemplate Address
	return json.Unmarshal(body, (*addressTemplate)(a))
}

type Strategy struct {
	Interval types.Duration `json:"interval"`

	Addresses map[string]Address `json:"addresses"`

	MaxDailyNumberOfTransfer int              `json:"maxDailyNumberOfTransfer"`
	MaxDailyAmountOfTransfer fixedpoint.Value `json:"maxDailyAmountOfTransfer"`

	CheckOnStart bool `json:"checkOnStart"`

	Asset string `json:"asset"`

	// Low is the low balance level for triggering transfer
	Low fixedpoint.Value `json:"low"`

	// Middle is the middle balance level used for re-fill asset
	Middle fixedpoint.Value `json:"middle"`

	Verbose bool `json:"verbose"`

	State *State `persistence:"state"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) checkBalance(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	if s.Verbose {
		bbgo.Notify("📝 Checking %s low balance level exchange session...", s.Asset)
	}

	var total fixedpoint.Value
	for _, session := range sessions {
		if b, ok := session.GetAccount().Balance(s.Asset); ok {
			total = total.Add(b.Total())
		}
	}

	lowLevelSession, lowLevelBalance, err := s.findLowBalanceLevelSession(sessions)
	if err != nil {
		bbgo.Notify("Can not find low balance level session: %s", err.Error())
		log.WithError(err).Errorf("Can not find low balance level session")
		return
	}

	if lowLevelSession == nil {
		if s.Verbose {
			bbgo.Notify("✅ All %s balances are looking good, total value: %v", s.Asset, total)
		}
		return
	}

	bbgo.Notify("⚠️ Found low level %s balance from session %s: %v", s.Asset, lowLevelSession.Name, lowLevelBalance)

	middle := s.Middle
	if middle.IsZero() {
		middle = total.Div(fixedpoint.NewFromInt(int64(len(sessions)))).Mul(priceFixer)
		bbgo.Notify("Total value %v %s, setting middle to %v", total, s.Asset, middle)
	}

	requiredAmount := middle.Sub(lowLevelBalance.Available)

	bbgo.Notify("Need %v %s to satisfy the middle balance level %v", requiredAmount, s.Asset, middle)

	fromSession, _, err := s.findHighestBalanceLevelSession(sessions, requiredAmount)
	if err != nil || fromSession == nil {
		bbgo.Notify("Can not find session with enough balance")
		log.WithError(err).Errorf("can not find session with enough balance")
		return
	}

	withdrawalService, ok := fromSession.Exchange.(types.ExchangeWithdrawalService)
	if !ok {
		log.Errorf("exchange %s does not implement withdrawal service, we can not withdrawal", fromSession.ExchangeName)
		return
	}

	if !fromSession.Withdrawal {
		bbgo.Notify("The withdrawal function exchange session %s is not enabled", fromSession.Name)
		log.Errorf("The withdrawal function of exchange session %s is not enabled", fromSession.Name)
		return
	}

	toAddress, ok := s.Addresses[lowLevelSession.Name]
	if !ok {
		log.Errorf("%s address of session %s not found", s.Asset, lowLevelSession.Name)
		bbgo.Notify("%s address of session %s not found", s.Asset, lowLevelSession.Name)
		return
	}

	if toAddress.ForeignFee.Sign() > 0 {
		requiredAmount = requiredAmount.Add(toAddress.ForeignFee)
	}

	if s.State != nil {
		if s.MaxDailyNumberOfTransfer > 0 {
			if s.State.DailyNumberOfTransfers >= s.MaxDailyNumberOfTransfer {
				bbgo.Notify("⚠️ Exceeded %s max daily number of transfers %d (current %d), skipping transfer...",
					s.Asset,
					s.MaxDailyNumberOfTransfer,
					s.State.DailyNumberOfTransfers)
				return
			}
		}

		if s.MaxDailyAmountOfTransfer.Sign() > 0 {
			if s.State.DailyAmountOfTransfers.Compare(s.MaxDailyAmountOfTransfer) >= 0 {
				bbgo.Notify("⚠️ Exceeded %s max daily amount of transfers %v (current %v), skipping transfer...",
					s.Asset,
					s.MaxDailyAmountOfTransfer,
					s.State.DailyAmountOfTransfers)
				return
			}
		}
	}

	bbgo.Notify(&WithdrawalRequest{
		FromSession: fromSession.Name,
		ToSession:   lowLevelSession.Name,
		Asset:       s.Asset,
		Amount:      requiredAmount,
	})

	if err := withdrawalService.Withdraw(ctx, s.Asset, requiredAmount, toAddress.Address, &types.WithdrawalOptions{
		Network:    toAddress.Network,
		AddressTag: toAddress.AddressTag,
	}); err != nil {
		log.WithError(err).Errorf("withdrawal failed")
		bbgo.Notify("withdrawal request failed, error: %v", err)
		return
	}

	bbgo.Notify("%s withdrawal request sent", s.Asset)

	if s.State != nil {
		if s.State.IsOver24Hours() {
			s.State.Reset()
		}

		s.State.DailyNumberOfTransfers += 1
		s.State.DailyAmountOfTransfers = s.State.DailyAmountOfTransfers.Add(requiredAmount)
		bbgo.Sync(ctx, s)
	}
}

func (s *Strategy) findHighestBalanceLevelSession(sessions map[string]*bbgo.ExchangeSession, requiredAmount fixedpoint.Value) (*bbgo.ExchangeSession, types.Balance, error) {
	var balance types.Balance
	var maxBalanceLevel = fixedpoint.Zero
	var maxBalanceSession *bbgo.ExchangeSession = nil
	for sessionID := range s.Addresses {
		session, ok := sessions[sessionID]
		if !ok {
			return nil, balance, fmt.Errorf("session %s does not exist", sessionID)
		}

		if b, ok := session.GetAccount().Balance(s.Asset); ok {
			if b.Available.Sub(requiredAmount).Compare(s.Low) > 0 && b.Available.Compare(maxBalanceLevel) > 0 {
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

		balance, ok = session.GetAccount().Balance(s.Asset)
		if ok {
			if balance.Available.Compare(s.Low) <= 0 {
				return session, balance, nil
			}
		}
	}

	return nil, balance, nil
}

func (s *Strategy) newDefaultState() *State {
	return &State{
		Asset:                  s.Asset,
		DailyNumberOfTransfers: 0,
		DailyAmountOfTransfers: fixedpoint.Zero,
	}
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.Interval == 0 {
		return errors.New("interval can not be zero")
	}

	if s.State == nil {
		s.State = s.newDefaultState()
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
	})

	if s.CheckOnStart {
		s.checkBalance(ctx, sessions)
	}

	go func() {
		ticker := time.NewTicker(util.MillisecondsJitter(s.Interval.Duration(), 1000))
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
