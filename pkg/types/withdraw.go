package types

import (
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Withdraw struct {
	GID        int64            `json:"gid" db:"gid"`
	Exchange   ExchangeName     `json:"exchange" db:"exchange"`
	Asset      string           `json:"asset" db:"asset"`
	Amount     fixedpoint.Value `json:"amount" db:"amount"`
	Address    string           `json:"address" db:"address"`
	AddressTag string           `json:"addressTag"`
	Status     string           `json:"status"`

	TransactionID          string           `json:"transactionID" db:"txn_id"`
	TransactionFee         fixedpoint.Value `json:"transactionFee" db:"txn_fee"`
	TransactionFeeCurrency string           `json:"transactionFeeCurrency" db:"txn_fee_currency"`
	WithdrawOrderID        string           `json:"withdrawOrderId"`
	ApplyTime              Time             `json:"applyTime" db:"time"`
	Network                string           `json:"network" db:"network"`
}

func cutstr(s string, maxLen, head, tail int) string {
	if len(s) > maxLen {
		l := len(s)
		return s[0:head] + "..." + s[l-tail:]
	}
	return s
}

func (w Withdraw) String() (o string) {
	o = fmt.Sprintf("%s WITHDRAW %8f %s -> ", w.Exchange, w.Amount.Float64(), w.Asset)

	if len(w.Network) > 0 && w.Network != w.Asset {
		o += w.Network + ":"
	}

	o += fmt.Sprintf("%s @ %s", w.Address, w.ApplyTime.Time())

	if !w.TransactionFee.IsZero() {
		feeCurrency := w.TransactionFeeCurrency
		if feeCurrency == "" {
			feeCurrency = w.Asset
		}

		o += fmt.Sprintf(" FEE %4f %5s", w.TransactionFee.Float64(), feeCurrency)
	}

	if len(w.TransactionID) > 0 {
		o += fmt.Sprintf(" TxID: %s", cutstr(w.TransactionID, 12, 4, 4))
	}

	return o
}

func (w Withdraw) EffectiveTime() time.Time {
	return w.ApplyTime.Time()
}

type WithdrawalOptions struct {
	Network    string
	AddressTag string
}
