package types

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype"
)

type Withdraw struct {
	GID        int64        `json:"gid" db:"gid"`
	Exchange   ExchangeName `json:"exchange" db:"exchange"`
	Asset      string       `json:"asset" db:"asset"`
	Amount     float64      `json:"amount" db:"amount"`
	Address    string       `json:"address" db:"address"`
	AddressTag string       `json:"addressTag"`
	Status     string       `json:"status"`

	TransactionID          string        `json:"transactionID" db:"txn_id"`
	TransactionFee         float64       `json:"transactionFee" db:"txn_fee"`
	TransactionFeeCurrency string        `json:"transactionFeeCurrency" db:"txn_fee_currency"`
	WithdrawOrderID        string        `json:"withdrawOrderId"`
	ApplyTime              datatype.Time `json:"applyTime" db:"time"`
	Network                string        `json:"network" db:"network"`
}

func (w Withdraw) EffectiveTime() time.Time {
	return w.ApplyTime.Time()
}
