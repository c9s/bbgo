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
	"github.com/c9s/bbgo/pkg/fixedpoint"
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

	Interval types.Duration `json:"interval"`

	marginTransferService marginTransferService
	depositHistoryService types.ExchangeTransferService

	session          *bbgo.ExchangeSession
	watchingDeposits map[string]types.Deposit
	mu               sync.Mutex

	lastAssetDepositTimes map[string]time.Time
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Interval == 0 {
		s.Interval = types.Duration(5 * time.Minute)
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

	var ok bool

	s.marginTransferService, ok = session.Exchange.(marginTransferService)
	if !ok {
		return errMarginTransferNotSupport
	}

	s.depositHistoryService, ok = session.Exchange.(types.ExchangeTransferService)
	if !ok {
		return errDepositHistoryNotSupport
	}

	session.UserDataStream.OnStart(func() {
		go s.tickWatcher(ctx, s.Interval.Duration())
	})

	return nil
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
	accountLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

	for _, asset := range s.Assets {
		log.Infof("checking %s deposits...", asset)

		succeededDeposits, err := s.scanDepositHistory(ctx, asset, 4*time.Hour)
		if err != nil {
			log.WithError(err).Errorf("unable to scan deposit history")
			return
		}

		if len(succeededDeposits) == 0 {
			log.Infof("no %s deposit found", asset)
			continue
		}

		for _, d := range succeededDeposits {
			log.Infof("found succeeded deposit: %+v", d)

			if err2 := accountLimiter.Wait(ctx); err2 != nil {
				log.WithError(err2).Errorf("rate limiter error")
				return
			}

			// we can't use the account from margin
			amount := d.Amount
			if service, ok := s.session.Exchange.(spotAccountQueryService); ok {
				account, err2 := service.QuerySpotAccount(ctx)
				if err2 != nil {
					log.WithError(err2).Errorf("unable to query spot account")
					continue
				}

				if bal, ok := account.Balance(d.Asset); ok {
					log.Infof("spot account balance %s: %+v", d.Asset, bal)
					amount = fixedpoint.Min(bal.Available, amount)
				} else {
					log.Errorf("unexpected error: %s balance not found", d.Asset)
				}
			}

			bbgo.Notify("Found succeeded deposit %s %s, transferring %s %s into the margin account",
				d.Amount.String(), d.Asset,
				amount.String(), d.Asset)

			if err2 := s.marginTransferService.TransferMarginAccountAsset(ctx, d.Asset, amount, types.TransferIn); err2 != nil {
				log.WithError(err2).Errorf("unable to transfer deposit asset into the margin account")
			}
		}
	}
}

func (s *Strategy) scanDepositHistory(ctx context.Context, asset string, duration time.Duration) ([]types.Deposit, error) {
	log.Infof("scanning %s deposit history...", asset)

	now := time.Now()
	since := now.Add(-duration)
	deposits, err := s.depositHistoryService.QueryDepositHistory(ctx, asset, since, now)
	if err != nil {
		return nil, err
	}

	// sort the recent deposit records in ascending order
	sort.Slice(deposits, func(i, j int) bool {
		return deposits[i].Time.Time().Before(deposits[j].Time.Time())
	})

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, deposit := range deposits {
		log.Infof("checking deposit: %+v", deposit)

		if deposit.Asset != asset {
			continue
		}

		if _, ok := s.watchingDeposits[deposit.TransactionID]; ok {
			// if the deposit record is in the watch list, update it
			s.watchingDeposits[deposit.TransactionID] = deposit
		} else {
			switch deposit.Status {

			case types.DepositSuccess:
				if depositTime, ok := s.lastAssetDepositTimes[asset]; ok {
					// if it's newer than the latest deposit time, then we just add it the monitoring list
					if deposit.Time.After(depositTime) {
						log.Infof("adding new success deposit: %s", deposit.TransactionID)
						s.watchingDeposits[deposit.TransactionID] = deposit
					}
				} else {
					// ignore all initial deposit history that are already success
					log.Infof("ignored succeess deposit: %s %+v", deposit.TransactionID, deposit)
				}

			case types.DepositCredited, types.DepositPending:
				log.Infof("adding pending deposit: %s", deposit.TransactionID)
				s.watchingDeposits[deposit.TransactionID] = deposit
			}
		}
	}

	if len(deposits) > 0 {
		lastDeposit := deposits[len(deposits)-1]
		if lastTime, ok := s.lastAssetDepositTimes[asset]; ok {
			s.lastAssetDepositTimes[asset] = later(lastDeposit.Time.Time(), lastTime)
		} else {
			s.lastAssetDepositTimes[asset] = lastDeposit.Time.Time()
		}
	}

	var succeededDeposits []types.Deposit
	for _, deposit := range s.watchingDeposits {
		if deposit.Status == types.DepositSuccess {
			log.Infof("found pending -> success deposit: %+v", deposit)

			current, required := deposit.GetCurrentConfirmation()
			if required > 0 && deposit.UnlockConfirm > 0 && current < deposit.UnlockConfirm {
				log.Infof("deposit %s unlock confirm %d is not reached, current: %d, required: %d, skip this round", deposit.TransactionID, deposit.UnlockConfirm, current, required)
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
