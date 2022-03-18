package xnav

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "xnav"

const stateKey = "state-v1"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Since int64 `json:"since"`
}

func (s *State) IsOver24Hours() bool {
	return util.Over24Hours(time.Unix(s.Since, 0))
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
		Fields: []slack.AttachmentField{},
		Footer: util.Render("Since {{ . }}", time.Unix(s.Since, 0).Format(time.RFC822)),
	}
}

func (s *State) Reset() {
	var beginningOfTheDay = util.BeginningOfTheDay(time.Now().Local())
	*s = State{
		Since: beginningOfTheDay.Unix(),
	}
}

type Strategy struct {
	Notifiability *bbgo.Notifiability
	*bbgo.Graceful
	*bbgo.Persistence

	Interval      types.Duration `json:"interval"`
	ReportOnStart bool           `json:"reportOnStart"`
	IgnoreDusts   bool           `json:"ignoreDusts"`
	state         *State
}

func (s *Strategy) ID() string {
	return ID
}

var Ten = fixedpoint.NewFromInt(10)

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) recordNetAssetValue(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	totalAssets := types.AssetMap{}
	totalBalances := types.BalanceMap{}
	lastPrices := map[string]fixedpoint.Value{}
	for _, session := range sessions {
		balances := session.Account.Balances()
		if err := session.UpdatePrices(ctx); err != nil {
			log.WithError(err).Error("price update failed")
			return
		}

		for _, b := range balances {
			if tb, ok := totalBalances[b.Currency]; ok {
				tb.Available = tb.Available.Add(b.Available)
				tb.Locked = tb.Locked.Add(b.Locked)
				totalBalances[b.Currency] = tb
			} else {
				totalBalances[b.Currency] = b
			}
		}

		prices := session.LastPrices()
		for m, p := range prices {
			lastPrices[m] = p
		}
	}

	assets := totalBalances.Assets(lastPrices)
	for currency, asset := range assets {
		// calculated if it's dust only when InUSD (usd value) is defined.
		if s.IgnoreDusts && !asset.InUSD.IsZero() && asset.InUSD.Compare(Ten) < 0 {
			continue
		}

		totalAssets[currency] = asset
	}

	s.Notifiability.Notify(totalAssets)

	if s.state != nil {
		if s.state.IsOver24Hours() {
			s.state.Reset()
		}

		s.SaveState()
	}
}

func (s *Strategy) SaveState() {
	if err := s.Persistence.Save(s.state, ID, stateKey); err != nil {
		log.WithError(err).Errorf("%s can not save state: %+v", ID, s.state)
	} else {
		log.Infof("%s state is saved: %+v", ID, s.state)
		// s.Notifiability.Notify("%s %s state is saved", ID, s.Asset, s.state)
	}
}

func (s *Strategy) newDefaultState() *State {
	return &State{}
}

func (s *Strategy) LoadState() error {
	var state State
	if err := s.Persistence.Load(&state, ID, stateKey); err != nil {
		if err != service.ErrPersistenceNotExists {
			return err
		}

		s.state = s.newDefaultState()
		s.state.Reset()
	} else {
		// we loaded it successfully
		s.state = &state

		// update Asset name for legacy caches
		// s.state.Asset = s.Asset

		log.Infof("%s state is restored: %+v", ID, s.state)
		s.Notifiability.Notify("%s state is restored", ID, s.state)
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

	if s.ReportOnStart {
		s.recordNetAssetValue(ctx, sessions)
	}

	go func() {
		ticker := time.NewTicker(util.MillisecondsJitter(s.Interval.Duration(), 1000))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				s.recordNetAssetValue(ctx, sessions)
			}
		}
	}()

	return nil
}
