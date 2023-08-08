package deposit2transfer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type marginTransferService interface {
	TransferMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error
}

const ID = "deposit2transfer"

var log = logrus.WithField("strategy", ID)

var errNotBinanceExchange = errors.New("not binance exchange, currently only support binance exchange")

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

	Interval types.Interval `json:"interval"`

	binanceSpot *binance.Exchange

	marginTransferService marginTransferService

	depositHistoryService types.ExchangeTransferService

	lastDeposit *types.Deposit

	watchingDeposits map[string]types.Deposit
	mu               sync.Mutex
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Interval == "" {
		s.Interval = types.Interval1m
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
	s.watchingDeposits = make(map[string]types.Deposit)

	var ok bool

	s.marginTransferService, ok = session.Exchange.(marginTransferService)
	if !ok {
		return errMarginTransferNotSupport
	}

	s.depositHistoryService, ok = session.Exchange.(types.ExchangeTransferService)
	if !ok {
		return errDepositHistoryNotSupport
	}

	return nil
}

func (s *Strategy) tickWatcher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			for _, asset := range s.Assets {
				succeededDeposits, err := s.scanDepositHistory(ctx, asset, 4*time.Hour)
				if err != nil {
					log.WithError(err).Errorf("unable to scan deposit history")
					continue
				}

				for _, d := range succeededDeposits {
					log.Infof("found succeeded deposit: %+v", d)

					bbgo.Notify("Found succeeded deposit %s %s, transferring asset into the margin account", d.Amount.String(), d.Asset)
					if err2 := s.marginTransferService.TransferMarginAccountAsset(ctx, d.Asset, d.Amount, types.TransferIn); err2 != nil {
						log.WithError(err2).Errorf("unable to transfer deposit asset into the margin account")
					}
				}
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

	// sort the recent deposit records in descending order
	sort.Slice(deposits, func(i, j int) bool {
		return deposits[i].Time.Time().Before(deposits[j].Time.Time())
	})

	s.mu.Lock()
	defer s.mu.Lock()

	for _, deposit := range deposits {
		if deposit.Asset != asset {
			continue
		}

		if _, ok := s.watchingDeposits[deposit.TransactionID]; ok {
			// if the deposit record is in the watch list, update it
			s.watchingDeposits[deposit.TransactionID] = deposit
		} else {
			switch deposit.Status {
			case types.DepositSuccess:
				// ignore all deposits that are already success
				continue

			case types.DepositCredited, types.DepositPending:
				s.watchingDeposits[deposit.TransactionID] = deposit
			}
		}
	}

	var succeededDeposits []types.Deposit
	for _, deposit := range deposits {
		if deposit.Status == types.DepositSuccess {
			current, required := deposit.GetCurrentConfirmation()
			if required > 0 && deposit.UnlockConfirm > 0 && current < deposit.UnlockConfirm {
				continue
			}

			succeededDeposits = append(succeededDeposits, deposit)
			delete(s.watchingDeposits, deposit.TransactionID)
		}
	}

	return succeededDeposits, nil
}
