package deposit2transfer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

type marginTransferService interface {
	TransferMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error
}

type spotAccountQueryService interface {
	QuerySpotAccount(ctx context.Context) (*types.Account, error)
}

const ID = "deposit2transfer"

var log = logrus.WithField("strategy", ID)

var errMarginTransferNotSupport = errors.New("exchange session does not support margin transfer")

var errDepositHistoryNotSupport = errors.New("exchange session does not support deposit history query")

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	Assets []string `json:"assets"`

	Interval      types.Duration `json:"interval"`
	TransferDelay types.Duration `json:"transferDelay"`

	IgnoreDust  bool                        `json:"ignoreDust"`
	DustAmounts map[string]fixedpoint.Value `json:"dustAmounts"`

	AutoRepay bool `json:"autoRepay"`

	SlackAlert *slackalert.SlackAlert `json:"slackAlert"`

	marginTransferService    marginTransferService
	marginBorrowRepayService types.MarginBorrowRepayService

	depositHistoryService types.ExchangeTransferService

	session *bbgo.ExchangeSession

	watchingDeposits map[string]types.Deposit
	mu               sync.Mutex

	logger logrus.FieldLogger

	lastAssetDepositTimes map[string]time.Time
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Interval == 0 {
		s.Interval = types.Duration(3 * time.Minute)
	}

	if s.TransferDelay == 0 {
		s.TransferDelay = types.Duration(3 * time.Second)
	}

	if s.DustAmounts == nil {
		s.DustAmounts = map[string]fixedpoint.Value{
			"USDC": fixedpoint.NewFromFloat(1.0),
			"USDT": fixedpoint.NewFromFloat(1.0),
			"BTC":  fixedpoint.NewFromFloat(0.00001),
			"ETH":  fixedpoint.NewFromFloat(0.00001),
		}
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if s.logger == nil {
		s.logger = log.Dup()
	}

	return nil
}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Assets)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.session = session
	s.watchingDeposits = make(map[string]types.Deposit)
	s.lastAssetDepositTimes = make(map[string]time.Time)
	s.logger = s.logger.WithField("exchange", session.ExchangeName)

	var ok bool

	s.marginTransferService, ok = session.Exchange.(marginTransferService)
	if !ok {
		return errMarginTransferNotSupport
	}

	s.marginBorrowRepayService, _ = session.Exchange.(types.MarginBorrowRepayService)

	s.depositHistoryService, ok = session.Exchange.(types.ExchangeTransferService)
	if !ok {
		return errDepositHistoryNotSupport
	}

	session.UserDataStream.OnStart(func() {
		go s.tickWatcher(ctx, s.Interval.Duration())
	})

	return nil
}

func (s *Strategy) isDust(asset string, amount fixedpoint.Value) bool {
	if s.IgnoreDust {
		if dustAmount, ok := s.DustAmounts[asset]; ok {
			return amount.Compare(dustAmount) <= 0
		}
	}

	return false
}

func (s *Strategy) tickWatcher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.checkDeposits(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.checkDeposits(ctx)
		}
	}
}

func (s *Strategy) checkDeposits(ctx context.Context) {
	accountLimiter := rate.NewLimiter(rate.Every(5*time.Second), 1)

	for _, asset := range s.Assets {
		logger := s.logger.WithField("asset", asset)

		logger.Debugf("checking %s deposits...", asset)

		succeededDeposits, err := s.scanDepositHistory(ctx, asset, 4*time.Hour)
		if err != nil {
			logger.WithError(err).Errorf("unable to scan deposit history")
			return
		}

		if len(succeededDeposits) == 0 {
			logger.Debugf("no %s deposit found", asset)
			continue
		} else {
			logger.Infof("found %d %s deposits", len(succeededDeposits), asset)
		}

		if s.TransferDelay > 0 {
			logger.Infof("delaying transfer for %s...", s.TransferDelay.Duration())
			time.Sleep(s.TransferDelay.Duration())
		}

		for _, d := range succeededDeposits {
			logger.Infof("found succeeded %s deposit: %+v", asset, d)

			if err2 := accountLimiter.Wait(ctx); err2 != nil {
				logger.WithError(err2).Errorf("rate limiter error")
				return
			}

			// we can't use the account from margin
			amount := d.Amount
			if service, ok := s.session.Exchange.(spotAccountQueryService); ok {
				var account *types.Account
				err = retry.GeneralBackoff(ctx, func() (err error) {
					account, err = service.QuerySpotAccount(ctx)
					return err
				})

				if err != nil || account == nil {
					logger.WithError(err).Errorf("unable to query spot account")
					continue
				}

				bal, ok := account.Balance(d.Asset)
				if ok {
					logger.Infof("spot account balance %s: %+v", d.Asset, bal)
					amount = fixedpoint.Min(bal.Available, amount)
				} else {
					logger.Errorf("unexpected error: %s balance not found", d.Asset)
				}

				if amount.IsZero() || amount.Sign() < 0 {
					bbgo.Notify("🔍 Found succeeded deposit %s %s on %s, but the balance %s %s is insufficient, skip transferring",
						d.Amount.String(), d.Asset,
						s.session.Name,
						bal.Available.String(), bal.Currency)
					continue
				}
			}

			bbgo.Notify("🔍 Found succeeded deposit %s %s on %s, transferring %s %s into the margin account",
				d.Amount.String(), d.Asset,
				s.session.Name,
				amount.String(), d.Asset)

			s.postLiveNoteMessage(d, "🚥 Transferring deposit asset %s %s into the margin account", amount.String(), d.Asset)

			err2 := retry.GeneralBackoff(ctx, func() error {
				return s.marginTransferService.TransferMarginAccountAsset(ctx, d.Asset, amount, types.TransferIn)
			})

			if err2 != nil {
				logger.WithError(err2).Errorf("unable to transfer deposit asset into the margin account")

				s.postLiveNoteError(d, "❌ Unable to transfer deposit asset into the margin account, error: %+v", err2)
			} else {
				s.logger.Infof("%s %s has been transferred successfully", amount.String(), d.Asset)

				s.postLiveNoteMessage(d, "✅ %s %s has been transferred successfully", amount.String(), d.Asset)

				if s.AutoRepay && s.marginBorrowRepayService != nil {
					s.logger.Infof("autoRepay is enabled, repaying %s %s...", amount.String(), d.Asset)

					time.Sleep(3 * time.Second)

					if err2 := s.repay(ctx, d, amount); err2 != nil {
						s.logger.WithError(err2).Errorf("unable to repay the margin asset")
					}
				}
			}
		}
	}
}

func (s *Strategy) repay(ctx context.Context, d types.Deposit, amount fixedpoint.Value) error {
	if amount.IsZero() {
		return nil
	}

	if amount.Sign() < 0 {
		return fmt.Errorf("invalid repay amount: %s", amount.String())
	}

	if !s.session.Margin {
		return fmt.Errorf("session does not support margin")
	}

	bals, err := s.session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	bal, ok := bals[d.Asset]
	if !ok {
		return nil
	}

	s.logger.Infof("found %s balance debt: %s", d.Asset, bal.Debt().String())

	amount = fixedpoint.Min(bal.Debt(),
		fixedpoint.Max(amount, bal.Available))

	if amount.Sign() <= 0 {
		s.logger.Infof("%s has no debt to repay", d.Asset)
		return nil
	}

	s.logger.Infof("adjusted repay amount to %s", amount.String())

	if err := s.marginBorrowRepayService.RepayMarginAsset(ctx, d.Asset, amount); err != nil {
		s.postLiveNoteError(d, "❌ Unable to repay, error: %+v", err)
		return err
	}

	s.logger.Infof("%s %s was successfully repaid", amount.String(), d.Asset)
	s.postLiveNoteMessage(d, "✅ %s %s was successfully repaid", amount.String(), d.Asset)
	return nil
}

func (s *Strategy) postLiveNoteError(d types.Deposit, msgf string, err error) {
	if s.SlackAlert != nil {
		bbgo.PostLiveNote(&d,
			livenote.Channel(s.SlackAlert.Channel),
			livenote.Comment(fmt.Sprintf(msgf, err)),
		)
	}
}

func (s *Strategy) postLiveNoteMessage(d types.Deposit, msgf string, args ...any) {
	if s.SlackAlert != nil {
		bbgo.PostLiveNote(&d,
			livenote.Channel(s.SlackAlert.Channel),
			livenote.Comment(fmt.Sprintf(msgf, args...)),
		)
	}
}

func (s *Strategy) addWatchingDeposit(deposit types.Deposit) {
	s.watchingDeposits[deposit.TransactionID] = deposit

	if lastTime, ok := s.lastAssetDepositTimes[deposit.Asset]; ok {
		s.lastAssetDepositTimes[deposit.Asset] = later(deposit.Time.Time(), lastTime)
	} else {
		s.lastAssetDepositTimes[deposit.Asset] = deposit.Time.Time()
	}

	if s.SlackAlert != nil {
		bbgo.PostLiveNote(&deposit,
			livenote.Channel(s.SlackAlert.Channel),
			livenote.Pin(s.SlackAlert.Pin),
			livenote.CompareObject(true),
			livenote.OneTimeMention(s.SlackAlert.Mentions...),
		)
	}
}

func (s *Strategy) scanDepositHistory(ctx context.Context, asset string, duration time.Duration) ([]types.Deposit, error) {
	logger := s.logger.WithField("asset", asset)
	logger.Debugf("scanning %s deposit history...", asset)

	now := time.Now()
	since := now.Add(-duration)

	var deposits []types.Deposit
	err := retry.GeneralBackoff(ctx, func() (err error) {
		deposits, err = s.depositHistoryService.QueryDepositHistory(ctx, asset, since, now)
		return err
	})

	if err != nil {
		return nil, err
	}

	// sort the recent deposit records in ascending order
	sort.Slice(deposits, func(i, j int) bool {
		return deposits[i].Time.Time().Before(deposits[j].Time.Time())
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	// update the watching deposits
	for _, deposit := range deposits {
		logger.Debugf("checking deposit: %+v", deposit)

		if deposit.Asset != asset {
			continue
		}

		if s.isDust(asset, deposit.Amount) {
			continue
		}

		// if the deposit record is already in the watch list, update it
		if _, ok := s.watchingDeposits[deposit.TransactionID]; ok {
			s.addWatchingDeposit(deposit)
		} else {
			// if the deposit record is not in the watch list, we need to check the status
			// here the deposit is outside the watching list
			switch deposit.Status {

			case types.DepositSuccess:
				// if the deposit is in success status, we need to check if it's newer than the latest deposit time
				// this usually happens when the deposit is credited to the account very quickly
				if depositTime, ok := s.lastAssetDepositTimes[asset]; ok {
					// if it's newer than the latest deposit time, then we just add it the monitoring list
					if deposit.Time.After(depositTime) {
						logger.Infof("adding new succeedded deposit: %s", deposit.TransactionID)
						s.addWatchingDeposit(deposit)
					} else {
						// ignore all initial deposits that are already in success status
						logger.Infof("ignored expired succeedded deposit: %s %+v", deposit.TransactionID, deposit)
					}
				} else {
					// if the latest deposit time is not found, check if the deposit is older than 5 minutes
					expiryTime := 5 * time.Minute
					if deposit.Time.Before(time.Now().Add(-expiryTime)) {
						logger.Infof("ignored expired (%s) succeedded deposit: %s %+v", expiryTime, deposit.TransactionID, deposit)
					} else {
						s.addWatchingDeposit(deposit)
					}
				}

			case types.DepositCredited, types.DepositPending:
				logger.Infof("adding pending deposit: %s", deposit.TransactionID)
				s.addWatchingDeposit(deposit)
			}
		}
	}

	var succeededDeposits []types.Deposit

	// find and move out succeeded deposits
	for _, deposit := range s.watchingDeposits {
		switch deposit.Status {
		case types.DepositSuccess:
			logger.Infof("found pending -> success deposit: %+v", deposit)
			current, required := deposit.GetCurrentConfirmation()
			if deposit.UnlockConfirm > 0 {
				if current < deposit.UnlockConfirm {
					logger.Infof("deposit %s unlock confirm %d is not reached, current: %d, required: %d, skip this round", deposit.TransactionID, deposit.UnlockConfirm, current, required)
					continue
				}
			} else if required > 0 && current < required {
				logger.Infof("deposit %s confirm %d/%d is not reached", deposit.TransactionID, current, required)
				continue
			}

			succeededDeposits = append(succeededDeposits, deposit)
			delete(s.watchingDeposits, deposit.TransactionID)
		}
	}

	return succeededDeposits, nil
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}

	return b
}
