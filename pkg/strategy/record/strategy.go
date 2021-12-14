package record

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

const ID = "record"

const stateKey = "record"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Since int64 `json:"since"`
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
	AccountService *service.AccountService

	Interval      types.Duration `json:"interval"`
	ReportOnStart bool           `json:"reportOnStart"`
	IgnoreDusts   bool           `json:"ignoreDusts"`
	state         *State
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) recordNetAssetValue(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	totalAssets := types.AssetMap{}
	totalBalances := types.BalanceMap{}
	lastPrices := map[string]float64{}

	notifyText := ""

	time := time.Now().Local()

	for _, session := range sessions {

		exBalances, err := session.Exchange.QueryAccountBalances(ctx)
		if err != nil {
			message := fmt.Sprintf("update balance fail: %s %s ", session.ExchangeName, session.SubAccount)
			s.Notifiability.Notify(message)
			log.WithError(err).Error(message)
			continue
		}
		session.Account.UpdateBalances(exBalances)

		balances := session.Account.Balances()
		var sessionBalances = types.BalanceMap{}

		if err := session.UpdatePrices(ctx); err != nil {
			log.WithError(err).Error("price update failed")
			return
		}

		for _, b := range balances {
			if tb, ok := sessionBalances[b.Currency]; ok {
				tb.Available += b.Available
				tb.Locked += b.Locked
				sessionBalances[b.Currency] = tb
			} else {
				sessionBalances[b.Currency] = b
			}
		}

		for _, b := range balances {
			if tb, ok := totalBalances[b.Currency]; ok {
				tb.Available += b.Available
				tb.Locked += b.Locked
				totalBalances[b.Currency] = tb
			} else {
				totalBalances[b.Currency] = b
			}
		}

		prices := session.LastPrices()
		for m, p := range prices {
			lastPrices[m] = p
		}

		assets := sessionBalances.Assets(lastPrices)
		var sessionAssets = types.AssetMap{}
		for _, asset := range assets {
			if s.IgnoreDusts && asset.InUSD < fixedpoint.NewFromFloat(10.0) {
				continue
			}
			sessionAssets[asset.Currency] = asset
		}
		s.AccountService.InsertAsset(time, session.ExchangeName, session.SubAccount, sessionAssets)

		notifyText += fmt.Sprintf("-----------%s--%s----------\n%s\n",
			session.ExchangeName, session.SubAccount, sessionAssets.PlainText())

		//log.Infof("%s sessionAssets: %s", ID, sessionAssets)

	}

	assets := totalBalances.Assets(lastPrices)
	for currency, asset := range assets {
		if s.IgnoreDusts && asset.InUSD < fixedpoint.NewFromFloat(10.0) {
			continue
		}

		totalAssets[currency] = asset
	}

	log.Infof("%s sessionAssets: %s", ID, totalAssets)

	notifyText += fmt.Sprintf("-----------all----------\n%s",
		totalAssets.PlainText())
	s.Notifiability.Notify(notifyText)

	// s.Notifiability.Notify(totalAssets)

	// if s.state != nil {
	// 	if s.state.IsOver24Hours() {
	// 		s.state.Reset()
	// 	}

	// 	s.SaveState()
	// }
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
