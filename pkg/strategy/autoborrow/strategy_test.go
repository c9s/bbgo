package autoborrow

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"DOGE": Number(10000)})
		account := newTestAccount(Number(10), Balance("USDT", Number(100)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "DOGE", Low: Number(1000), MaxTotalBorrow: Number(300)})

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(300), service.borrowed["DOGE"])
	})

	t.Run("max borrowable", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"DOGE": Number(200)})
		account := newTestAccount(Number(10), Balance("USDT", Number(100)))
		s := newTestStrategy(t, account, service,
			MarginAssetConfig{Asset: "DOGE", Low: Number(1000)})

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(200), service.borrowed["DOGE"])
	})
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
