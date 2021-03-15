package types

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype"
)

type DepositStatus string

const (
	DepositOther = DepositStatus("")

	DepositPending = DepositStatus("pending")

	DepositRejected = DepositStatus("rejected")

	DepositSuccess = DepositStatus("success")

	DepositCancelled = DepositStatus("canceled")

	// created but can not withdraw
	DepositCredited = DepositStatus("credited")
)

type Deposit struct {
	GID           int64         `json:"gid" db:"gid"`
	Exchange      ExchangeName  `json:"exchange" db:"exchange"`
	Time          datatype.Time `json:"time" db:"time"`
	Amount        float64       `json:"amount" db:"amount"`
	Asset         string        `json:"asset" db:"asset"`
	Address       string        `json:"address" db:"address"`
	AddressTag    string        `json:"addressTag"`
	TransactionID string        `json:"transactionID" db:"txn_id"`
	Status        DepositStatus `json:"status"`
}

func (d Deposit) EffectiveTime() time.Time {
	return d.Time.Time()
}
