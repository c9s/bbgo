package xnav

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/util/templateutil"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const ID = "xnav"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
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
	ReportOnStart bool           `json:"reportOnStart"`
	IgnoreDusts   bool           `json:"ignoreDusts"`

	State *State `persistence:"state"`
}

func (s *Strategy) ID() string {
	return ID
}

var Ten = fixedpoint.NewFromInt(10)

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) recordNetAssetValue(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	totalBalances := types.BalanceMap{}
	allPrices := map[string]fixedpoint.Value{}
	sessionBalances := map[string]types.BalanceMap{}
	priceTime := time.Now()

	// iterate the sessions and record them
	quoteCurrency := "USDT"
	for sessionName, session := range sessions {
		// update the account balances and the margin information
		if _, err := session.UpdateAccount(ctx); err != nil {
			log.WithError(err).Errorf("can not update account")
			return
		}

		account := session.GetAccount()
		balances := account.Balances()
		if err := session.UpdatePrices(ctx, balances.Currencies(), quoteCurrency); err != nil {
			log.WithError(err).Error("price update failed")
			return
		}

		sessionBalances[sessionName] = balances
		totalBalances = totalBalances.Add(balances)

		prices := session.LastPrices()
		assets := balances.Assets(prices, priceTime)

		// merge prices
		for m, p := range prices {
			allPrices[m] = p
		}

		s.Environment.RecordAsset(priceTime, session, assets)
	}

	displayAssets := types.AssetMap{}
	totalAssets := totalBalances.Assets(allPrices, priceTime)
	s.Environment.RecordAsset(priceTime, &bbgo.ExchangeSession{Name: "ALL"}, totalAssets)

	for currency, asset := range totalAssets {
		// calculated if it's dust only when InUSD (usd value) is defined.
		if s.IgnoreDusts && !asset.InUSD.IsZero() && asset.InUSD.Compare(Ten) < 0 && asset.InUSD.Compare(Ten.Neg()) > 0 {
			continue
		}

		displayAssets[currency] = asset
	}

	bbgo.Notify(displayAssets)

	if s.State != nil {
		if s.State.IsOver24Hours() {
			s.State.Reset()
		}
		bbgo.Sync(ctx, s)
	}
}

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	if s.Interval == "" {
		return errors.New("interval can not be empty")
	}

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
	}

	// TODO: if interval is supported, we can use kline as the ticker
	if _, ok := types.SupportedIntervals[s.Interval]; ok {

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
