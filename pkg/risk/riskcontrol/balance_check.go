package riskcontrol

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

type BalanceCheck struct {
	ExpectedBalances       map[string]fixedpoint.Value `json:"exceptedBalances"`
	BalanceCheckTorlerance fixedpoint.Value            `json:"balanceCheckTorlerance"`
}

func (c *BalanceCheck) Validate() error {
	if len(c.ExpectedBalances) == 0 {
		return fmt.Errorf("expectedBalances is empty")
	}

	if c.BalanceCheckTorlerance.IsZero() {
		return fmt.Errorf("balanceCheckTorlerance is zero")
	}

	for _, v := range c.ExpectedBalances {
		if v.IsZero() {
			return fmt.Errorf("expected balance is zero")
		}
	}
	return nil
}

func (c *BalanceCheck) Check(balances types.BalanceMap) error {
	for currency, exceptedBalance := range c.ExpectedBalances {
		b, ok := balances[currency]
		if !ok {
			return fmt.Errorf("balance of %s not found", currency)
		}

		// | (actual - expected) / expected | <= torlerance
		balanceErr := exceptedBalance.Sub(b.Available).Div(exceptedBalance).Abs()
		log.Infof("[BalanceCheck] %s, expected: %s, actual: %s, error: %.2f%%", currency, exceptedBalance, b.Available, balanceErr.Float64()*100)

		if balanceErr.Compare(c.BalanceCheckTorlerance) >= 0 {
			return fmt.Errorf("balance of %s is not matched, expected: %s, actual: %s, error: %.2f%%", currency, exceptedBalance, b.Available, balanceErr.Float64()*100)
		}
	}
	return nil
}
