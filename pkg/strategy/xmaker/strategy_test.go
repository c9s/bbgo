//go:build !dnum
// +build !dnum

package xmaker

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestStrategy_allowMarginHedge(t *testing.T) {
	symbol := "BTCUSDT"
	market := Market(symbol)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	priceSolver := pricesolver.NewSimplePriceResolver(AllMarkets())
	priceSolver.Update("BTCUSDT", Number(98000.0))
	priceSolver.Update("ETHUSDT", Number(3800.0))

	// total equity value = 2 BTC at 98,000 + 200,000.0 USDT = 196,000 USDT + 200,000.0 USDT = 396,000 USDT
	// net equity value = 1 BTC at 98,000 + 200,000.0 USDT = 98,000 USDT + 200,000.0 USDT = 298,000 USDT
	// debt value = 1 BTC at 98,000 = 98,000 USDT
	// current margin level = total equity value / debt value = 396,000 / 98,000 = 4.04081632653
	// borrowing quota = (total equity value / min margin level 1.7) - debt value = (396,000 / 1.7) - 98,000 = 232,941.176470588
	t.Run("safe margin level, calculate borrowing quota", func(t *testing.T) {
		account := types.NewAccount()
		account.MarginLevel = Number(3.04081632)
		account.SetBalance("BTC", types.Balance{
			Currency:  "BTC",
			Available: Number(2.0),
			Borrowed:  Number(1.0),
		})

		account.SetBalance("USDT", types.Balance{
			Currency:  "USDT",
			Available: Number(200_000.0),
		})

		session := &bbgo.ExchangeSession{
			Margin:  true,
			Account: account,
		}

		accountValueCalc := bbgo.NewAccountValueCalculator(session, priceSolver, currency.USDT)
		assert.Equal(t, "98000", accountValueCalc.DebtValue().String())
		assert.Equal(t, "298000", accountValueCalc.NetValue().String())

		s := &Strategy{
			MinMarginLevel:         Number(1.7),
			makerMarket:            market,
			sourceMarket:           market,
			sourceSession:          session,
			accountValueCalculator: accountValueCalc,
			logger:                 logger,
		}
		s.lastPrice.Set(Number(98000.0))

		allowed, quota := s.allowMarginHedge(types.SideTypeBuy)
		if assert.True(t, allowed) {
			assert.InDelta(t, 2.47735853175711e+06, quota.Float64(), 0.0001, "should be able to borrow %f USDT", quota.Float64())
		}

		allowed, quota = s.allowMarginHedge(types.SideTypeSell)
		if assert.True(t, allowed) {
			assert.InDelta(t, 25.27916869, quota.Float64(), 0.0001, "should be able to borrow %f BTC", quota.Float64())
		}
	})

	// total equity value = 2 BTC at 98,000 + 200,000.0 USDT = 196,000 USDT + 200,000.0 USDT = 396,000 USDT
	// net equity value = -2 BTC at 98,000 + 200,000.0 USDT = -196,000 USDT + 200,000.0 USDT = 4,000 USDT
	// debt value = 4 BTC at 98,000 = 392,000 USDT
	// current margin level = total equity value / debt value = 396,000 / 392,000 = 1.01020408163
	t.Run("low margin level, calculate quota", func(t *testing.T) {
		account := types.NewAccount()
		account.SetBalance("BTC", types.Balance{
			Currency:  "BTC",
			Available: Number(2.0),
			Borrowed:  Number(4.0),
		})

		account.SetBalance("USDT", types.Balance{
			Currency:  "USDT",
			Available: Number(200_000.0),
		})

		session := &bbgo.ExchangeSession{
			Margin:  true,
			Account: account,
		}

		accountValueCalc := bbgo.NewAccountValueCalculator(session, priceSolver, currency.USDT)
		assert.Equal(t, "392000", accountValueCalc.DebtValue().String())
		assert.Equal(t, "4000", accountValueCalc.NetValue().String())

		var err error
		account.MarginLevel, err = accountValueCalc.MarginLevel()
		if assert.NoError(t, err) {
			assert.InDelta(t, 1.01, account.MarginLevel.Float64(), 0.001)
		}

		s := &Strategy{
			MinMarginLevel:         Number(1.7),
			makerMarket:            market,
			sourceMarket:           market,
			sourceSession:          session,
			accountValueCalculator: accountValueCalc,
			logger:                 logger,
		}
		s.lastPrice.Set(Number(98000.0))

		allowed, quota := s.allowMarginHedge(types.SideTypeBuy)
		assert.False(t, allowed)

		allowed, quota = s.allowMarginHedge(types.SideTypeSell)
		if assert.True(t, allowed) {
			assert.InDelta(t, 2.04, quota.Float64(), 0.001, "should be able to borrow %f BTC", quota.Float64())
		}
	})
}

func Test_aggregatePrice(t *testing.T) {
	bids := PriceVolumeSliceFromText(`
	1000.0, 1.0
	1200.0, 1.0
	1400.0, 1.0
`)

	aggregatedPrice1 := aggregatePrice(bids, fixedpoint.NewFromFloat(0.5))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice1)

	aggregatedPrice2 := aggregatePrice(bids, fixedpoint.NewFromInt(1))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice2)

	aggregatedPrice3 := aggregatePrice(bids, fixedpoint.NewFromInt(2))
	assert.Equal(t, fixedpoint.NewFromFloat(1100.0), aggregatedPrice3)

}
