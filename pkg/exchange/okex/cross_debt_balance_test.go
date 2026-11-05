package okex

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// Ticket 03: a level-3 account carrying cross debt must surface that debt in
// the balance map so Debt() is non-zero. Spot-mode balances are unchanged.
func TestToGlobalBalance_CrossLiability(t *testing.T) {
	t.Run("cross liability is reported as borrowed debt", func(t *testing.T) {
		account := &okexapi.Account{
			Details: []okexapi.BalanceDetail{
				{
					Currency:      "BTC",
					Available:     fixedpoint.MustNewFromString("0.5"),
					FrozenBalance: fixedpoint.MustNewFromString("0.1"),
					// multi-currency margin (cross) reports the debt in crossLiab
					CrossLiab: fixedpoint.MustNewFromString("0.2"),
				},
			},
		}

		balances := toGlobalBalance(account)
		btc := balances["BTC"]

		assert.True(t, btc.Borrowed.Sign() > 0, "cross debt must be visible as borrowed")
		assert.Equal(t, fixedpoint.MustNewFromString("0.2"), btc.Borrowed)
		// Debt() = Borrowed + Interest
		assert.Equal(t, fixedpoint.MustNewFromString("0.2"), btc.Debt())
	})

	t.Run("currency liability and cross liability: the larger absolute value wins", func(t *testing.T) {
		account := &okexapi.Account{
			Details: []okexapi.BalanceDetail{
				{
					Currency:    "ETH",
					// spot borrow reports the debt in liab
					Liability: fixedpoint.MustNewFromString("-3.0"),
					CrossLiab: fixedpoint.MustNewFromString("1.0"),
				},
			},
		}

		balances := toGlobalBalance(account)
		eth := balances["ETH"]

		assert.Equal(t, fixedpoint.MustNewFromString("3.0"), eth.Borrowed)
	})

	t.Run("no liability at all: debt stays zero", func(t *testing.T) {
		account := &okexapi.Account{
			Details: []okexapi.BalanceDetail{
				{
					Currency:  "BTC",
					Available: fixedpoint.MustNewFromString("1.0"),
				},
			},
		}

		balances := toGlobalBalance(account)
		assert.True(t, balances["BTC"].Debt().IsZero())
	})

	t.Run("available and locked are preserved alongside the debt", func(t *testing.T) {
		account := &okexapi.Account{
			Details: []okexapi.BalanceDetail{
				{
					Currency:      "BTC",
					Available:     fixedpoint.MustNewFromString("0.5"),
					FrozenBalance: fixedpoint.MustNewFromString("0.1"),
					CrossLiab:     fixedpoint.MustNewFromString("0.2"),
					Equity:        fixedpoint.MustNewFromString("50000"),
				},
			},
		}

		balances := toGlobalBalance(account)
		btc := balances["BTC"]

		assert.Equal(t, fixedpoint.MustNewFromString("0.5"), btc.Available)
		assert.Equal(t, fixedpoint.MustNewFromString("0.1"), btc.Locked)
		assert.Equal(t, fixedpoint.MustNewFromString("50000"), btc.NetAsset)
		assert.Equal(t, "BTC", btc.Currency)
	})
}
