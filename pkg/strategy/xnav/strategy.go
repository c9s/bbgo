package xnav

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/asset"
	"github.com/c9s/bbgo/pkg/util/templateutil"
	"github.com/c9s/bbgo/pkg/util/timejitter"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

const ID = "xnav"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type AllAssetSnapshot struct {
	SessionAssets map[string]asset.Map
	TotalAssets   asset.Map
	Time          time.Time
}

type State struct {
	Since int64 `json:"since"`
}

func (s *State) IsOver24Hours() bool {
	return types.Over24Hours(time.Unix(s.Since, 0))
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
		Fields: []slack.AttachmentField{},
		Footer: templateutil.Render("Since {{ . }}", time.Unix(s.Since, 0).Format(time.RFC822)),
	}
}

func (s *State) Reset() {
	var beginningOfTheDay = types.BeginningOfTheDay(time.Now().Local())
	*s = State{
		Since: beginningOfTheDay.Unix(),
	}
}

type Strategy struct {
	*bbgo.Environment

	Interval      types.Interval `json:"interval"`
	Schedule      string         `json:"schedule"`
	ReportOnStart bool           `json:"reportOnStart"`
	IgnoreDusts   bool           `json:"ignoreDusts"`
	ShowBreakdown bool           `json:"showBreakdown"`

	ShowDebtDetails bool `json:"showDebtDetails"`

	lastAllAssetSnapshot *AllAssetSnapshot

	State *State `persistence:"state"`

	cron *cron.Cron
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Initialize() error {
	return nil
}

func (s *Strategy) Validate() error {
	if s.Interval == "" && s.Schedule == "" {
		return fmt.Errorf("interval or schedule is required")
	}
	return nil
}

var ten = fixedpoint.NewFromInt(10)

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) recordNetAssetValue(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	log.Infof("recording net asset value...")

	priceTime := time.Now()
	sessionAssets := map[string]asset.Map{}

	// iterate the sessions and record them
	quoteCurrency := "USDT"
	for sessionName, session := range sessions {
		log.Infof("recording net asset value for session %s...", sessionName)

		if session.PublicOnly {
			log.Infof("session %s is public only, skip", sessionName)
			continue
		}

		// update the account balances and the margin information
		if _, err := session.UpdateAccount(ctx); err != nil {
			log.WithError(err).Errorf("can not update account")
			return
		}

		account := session.GetAccount()
		balances := account.Balances().NotZero()
		if err := session.UpdatePrices(ctx, balances.Currencies(), quoteCurrency); err != nil {
			log.WithError(err).Error("price update failed")
			return
		}

		assets := NewAssetMapFromBalanceMap(session.GetPriceSolver(), priceTime, balances, quoteCurrency)
		s.Environment.RecordAsset(priceTime, session, assets)

		for _, as := range assets {
			log.WithFields(logrus.Fields{
				"session":  sessionName,
				"exchange": session.ExchangeName,
			}).Infof("session %s %s asset = net:%s available:%s",
				sessionName,
				as.Currency,
				as.NetAsset.String(),
				as.Available.String())
		}

		sessionAssets[sessionName] = assets

		if s.ShowBreakdown {
			slackAttachment := assets.SlackAttachment()
			slackAttachment.Title = "Session " + sessionName + " " + slackAttachment.Title
			bbgo.Notify(slackAttachment)
		}
	}

	totalAssets := asset.Map{}
	for _, assets := range sessionAssets {
		totalAssets = totalAssets.Merge(assets)
	}

	displayAssets := totalAssets.Filter(func(asset *asset.Asset) bool {
		if s.IgnoreDusts && asset.NetAssetInUSD.Abs().Compare(ten) < 0 && asset.DebtInUSD.Abs().Compare(ten) < 0 {
			return false
		}

		return true
	})

	s.Environment.RecordAsset(priceTime, &bbgo.ExchangeSession{Name: "ALL"}, totalAssets)

	bbgo.Notify(displayAssets)

	s.lastAllAssetSnapshot = &AllAssetSnapshot{
		SessionAssets: sessionAssets,
		TotalAssets:   totalAssets,
		Time:          priceTime,
	}

	if s.State != nil {
		if s.State.IsOver24Hours() {
			s.State.Reset()
		}
		bbgo.Sync(ctx, s)
	}
}

func (s *Strategy) worker(ctx context.Context, sessions map[string]*bbgo.ExchangeSession, interval time.Duration) {
	ticker := time.NewTicker(timejitter.Milliseconds(interval, 1000))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.recordNetAssetValue(ctx, sessions)
		}
	}
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.State == nil {
		s.State = &State{}
		s.State.Reset()
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		bbgo.Sync(ctx, s)
	})

	if s.ReportOnStart {
		s.recordNetAssetValue(ctx, sessions)
	}

	if s.Environment.BacktestService != nil {
		log.Warnf("xnav does not support backtesting")
		return nil
	}

	if s.Interval != "" {
		go s.worker(ctx, sessions, s.Interval.Duration())
	} else if s.Schedule != "" {
		s.cron = cron.New()
		_, err := s.cron.AddFunc(s.Schedule, func() {
			s.recordNetAssetValue(ctx, sessions)
		})
		if err != nil {
			return err
		}
		s.cron.Start()
	}

	return nil
}
