package types

import (
	"fmt"
	"time"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Withdraw struct {
	GID        int64        `json:"gid" db:"gid"`
	Exchange   ExchangeName `json:"exchange" db:"exchange"`
	Asset      string       `json:"asset" db:"asset"`
	Amount     fixedpoint.Value      `json:"amount" db:"amount"`
	Address    string       `json:"address" db:"address"`
	AddressTag string       `json:"addressTag"`
	Status     string       `json:"status"`

	TransactionID          string  `json:"transactionID" db:"txn_id"`
	TransactionFee         fixedpoint.Value `json:"transactionFee" db:"txn_fee"`
	TransactionFeeCurrency string  `json:"transactionFeeCurrency" db:"txn_fee_currency"`
	WithdrawOrderID        string  `json:"withdrawOrderId"`
	ApplyTime              Time    `json:"applyTime" db:"time"`
	Network                string  `json:"network" db:"network"`
}

func (w Withdraw) String() string {
	return fmt.Sprintf("withdraw %s %f to %s at %s", w.Asset, w.Amount, w.Address, w.ApplyTime.Time())
}

func (w Withdraw) EffectiveTime() time.Time {
	return w.ApplyTime.Time()
}

type WithdrawalOptions struct {
	Network    string
	AddressTag string
}

