package autoborrow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// fakeMarginBorrowRepayService records borrow and repay calls and returns
// preconfigured max borrowable amounts.
type fakeMarginBorrowRepayService struct {
	maxBorrowable map[string]fixedpoint.Value
	borrowed      map[string]fixedpoint.Value
	repaid        map[string]fixedpoint.Value
	assetInfo     map[string]types.MarginAssetInfo
	assetInfoErr  error
}

func (f *fakeMarginBorrowRepayService) QueryMarginAssetInfo(_ context.Context, asset string) (types.MarginAssetInfo, error) {
	if f.assetInfoErr != nil {
		return types.MarginAssetInfo{}, f.assetInfoErr
	}
	if info, ok := f.assetInfo[asset]; ok {
		return info, nil
	}
	return types.MarginAssetInfo{Asset: asset, IsBorrowable: true}, nil
}

func newFakeMarginBorrowRepayService(maxBorrowable map[string]fixedpoint.Value) *fakeMarginBorrowRepayService {
	return &fakeMarginBorrowRepayService{
		maxBorrowable: maxBorrowable,
		borrowed:      make(map[string]fixedpoint.Value),
		repaid:        make(map[string]fixedpoint.Value),
	}
}

func (f *fakeMarginBorrowRepayService) RepayMarginAsset(_ context.Context, asset string, amount fixedpoint.Value) error {
	f.repaid[asset] = amount
	return nil
}

func (f *fakeMarginBorrowRepayService) BorrowMarginAsset(_ context.Context, asset string, amount fixedpoint.Value) error {
	f.borrowed[asset] = amount
	return nil
}

func (f *fakeMarginBorrowRepayService) QueryMarginAssetMaxBorrowable(_ context.Context, asset string) (fixedpoint.Value, error) {
	return f.maxBorrowable[asset], nil
}

// newTestStrategy builds a strategy whose session always reports the given account.
func newTestStrategy(
	t *testing.T, account *types.Account, service *fakeMarginBorrowRepayService, assets ...MarginAssetConfig,
) *Strategy {
	mockCtrl := gomock.NewController(t)
	mockExchange := mocks.NewMockExchange(mockCtrl)
	mockExchange.EXPECT().QueryAccount(gomock.Any()).Return(account, nil).AnyTimes()

	return &Strategy{
		MinMarginLevel:    Number(1.5),
		Assets:            assets,
		ExchangeSession:   &bbgo.ExchangeSession{Exchange: mockExchange, Account: account},
		marginBorrowRepay: service,
		logger:            logrus.WithField("strategy", ID),
	}
}

func newTestAccount(marginLevel fixedpoint.Value, balances ...types.Balance) *types.Account {
	account := types.NewAccount()
	account.MarginLevel = marginLevel
	bm := types.BalanceMap{}
	for _, b := range balances {
		bm[b.Currency] = b
	}
	account.UpdateBalances(bm)
	return account
}

func debtBalance(currency string, available, borrowed fixedpoint.Value) types.Balance {
	b := types.NewZeroBalance(currency)
	b.Available = available
	b.Borrowed = borrowed
	return b
}

func TestCheckAndBorrow_UnheldAssetRespectsBorrowLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("max total borrow", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"XYZ": Number(10000)})
		account := newTestAccount(Number(10), Balance("USDT", Number(100)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "XYZ", Low: Number(1000), MaxTotalBorrow: Number(300)})

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(300), service.borrowed["XYZ"])
	})

	t.Run("max borrowable", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"XYZ": Number(200)})
		account := newTestAccount(Number(10), Balance("USDT", Number(100)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "XYZ", Low: Number(1000)})

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(200), service.borrowed["XYZ"])
	})
}

func TestCheckAndBorrow_MinQuantityPerBorrow(t *testing.T) {
	tests := []struct {
		name          string
		available     fixedpoint.Value
		borrowed      fixedpoint.Value
		maxPerBorrow  fixedpoint.Value
		maxBorrowable fixedpoint.Value
		minPerBorrow  fixedpoint.Value
		wantBorrow    fixedpoint.Value
	}{
		{
			name:      "small balance shortfall",
			available: Number(950), maxBorrowable: Number(10000),
			minPerBorrow: Number(100),
		},
		{
			name:         "per request cap below minimum",
			maxPerBorrow: Number(50), maxBorrowable: Number(10000),
			minPerBorrow: Number(100),
		},
		{
			name:     "remaining debt allowance below minimum",
			borrowed: Number(9950), maxBorrowable: Number(10000),
			minPerBorrow: Number(100),
		},
		{
			name:          "exchange maximum below minimum",
			maxBorrowable: Number(50), minPerBorrow: Number(100),
		},
		{
			name:      "exactly minimum",
			available: Number(900), maxBorrowable: Number(10000),
			minPerBorrow: Number(100), wantBorrow: Number(100),
		},
		{
			name:      "above minimum",
			available: Number(800), maxBorrowable: Number(10000),
			minPerBorrow: Number(100), wantBorrow: Number(200),
		},
		{
			name:      "minimum omitted preserves small borrows",
			available: Number(950), maxBorrowable: Number(10000),
			wantBorrow: Number(50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"XYZ": tt.maxBorrowable})
			account := newTestAccount(Number(10), debtBalance("XYZ", tt.available, tt.borrowed))
			s := newTestStrategy(t, account, service, MarginAssetConfig{
				Asset: "XYZ", Low: Number(1000), MaxTotalBorrow: Number(10000),
				MaxQuantityPerBorrow: tt.maxPerBorrow, MinQuantityPerBorrow: tt.minPerBorrow,
			})

			s.checkAndBorrow(context.Background())

			if tt.wantBorrow.IsZero() {
				assert.Empty(t, service.borrowed)
			} else {
				assert.Equal(t, tt.wantBorrow, service.borrowed["XYZ"])
			}
		})
	}
}

func TestCheckAndBorrow_ExchangeMinimum(t *testing.T) {
	tests := []struct {
		name          string
		available     fixedpoint.Value
		borrowed      fixedpoint.Value
		maxPerBorrow  fixedpoint.Value
		maxBorrowable fixedpoint.Value
		configuredMin fixedpoint.Value
		borrowable    bool
		queryErr      error
		wantBorrow    fixedpoint.Value
	}{
		{name: "shortfall below exchange minimum", available: Number(950), maxBorrowable: Number(10000), borrowable: true},
		{name: "per request cap below exchange minimum", maxPerBorrow: Number(50), maxBorrowable: Number(10000), borrowable: true},
		{name: "remaining debt allowance below exchange minimum", borrowed: Number(9950), maxBorrowable: Number(10000), borrowable: true},
		{name: "exchange cap below exchange minimum", maxBorrowable: Number(50), borrowable: true},
		{name: "configured floor cannot lower exchange minimum", available: Number(950), maxBorrowable: Number(10000), configuredMin: Number(10), borrowable: true},
		{name: "configured floor can raise exchange minimum", available: Number(850), maxBorrowable: Number(10000), configuredMin: Number(200), borrowable: true},
		{name: "exactly exchange minimum", available: Number(900), maxBorrowable: Number(10000), borrowable: true, wantBorrow: Number(100)},
		{name: "above exchange minimum", available: Number(800), maxBorrowable: Number(10000), borrowable: true, wantBorrow: Number(200)},
		{name: "not borrowable", maxBorrowable: Number(10000)},
		{name: "metadata query failure permits small borrow", available: Number(950), maxBorrowable: Number(10000), queryErr: errors.New("metadata unavailable"), wantBorrow: Number(50)},
		{name: "metadata query failure respects configured floor", available: Number(950), maxBorrowable: Number(10000), configuredMin: Number(100), queryErr: errors.New("metadata unavailable")},
		{name: "metadata query failure respects exchange cap", maxBorrowable: Number(200), queryErr: errors.New("metadata unavailable"), wantBorrow: Number(200)},
		{name: "metadata query failure respects debt cap", borrowed: Number(9950), maxBorrowable: Number(10000), queryErr: errors.New("metadata unavailable"), wantBorrow: Number(50)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"XYZ": tt.maxBorrowable})
			service.assetInfo = map[string]types.MarginAssetInfo{
				"XYZ": {Asset: "XYZ", IsBorrowable: tt.borrowable, MinBorrow: Number(100)},
			}
			service.assetInfoErr = tt.queryErr
			account := newTestAccount(Number(10), debtBalance("XYZ", tt.available, tt.borrowed))
			s := newTestStrategy(t, account, service, MarginAssetConfig{
				Asset: "XYZ", Low: Number(1000), MaxQuantityPerBorrow: tt.maxPerBorrow,
				MaxTotalBorrow: Number(10000), MinQuantityPerBorrow: tt.configuredMin,
			})
			logger, hook := logrustest.NewNullLogger()
			s.logger = logger

			s.checkAndBorrow(context.Background())

			if tt.queryErr != nil {
				assert.NotNil(t, hook.LastEntry())
				if entry := hook.LastEntry(); entry != nil {
					assert.Equal(t, logrus.WarnLevel, entry.Level)
					assert.Contains(t, entry.Message, "continuing with configured minimum borrow floor")
					assert.Equal(t, "XYZ", entry.Data["asset"])
				}
			}
			if tt.wantBorrow.IsZero() {
				assert.Empty(t, service.borrowed)
			} else {
				assert.Equal(t, tt.wantBorrow, service.borrowed["XYZ"])
			}
		})
	}
}

func TestCheckAndBorrow_MarginLevelBelowMin(t *testing.T) {
	defer func(d time.Duration) { repayRetryDelay = d }(repayRetryDelay)
	repayRetryDelay = 0

	ctx := context.Background()

	t.Run("skip borrowing when repaying cannot recover the margin level", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"USDT": Number(10000)})
		// no available balance, so the debt can not be repaid
		account := newTestAccount(Number(1.2), debtBalance("USDT", fixedpoint.Zero, Number(500)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "USDT", Low: Number(1000)})

		s.checkAndBorrow(ctx)

		assert.Empty(t, service.borrowed)
	})

	t.Run("borrow when the margin level is healthy", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"USDT": Number(10000)})
		account := newTestAccount(Number(3), debtBalance("USDT", fixedpoint.Zero, Number(500)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "USDT", Low: Number(1000)})

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(1000), service.borrowed["USDT"])
	})
}
