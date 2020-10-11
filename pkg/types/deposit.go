package types

import "time"

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
	Time          time.Time     `json:"time"`
	Amount        float64       `json:"amount"`
	Asset         string        `json:"asset"`
	Address       string        `json:"address"`
	AddressTag    string        `json:"addressTag"`
	TransactionID string        `json:"txId"`
	Status        DepositStatus `json:"status"`
}

func (d Deposit) EffectiveTime() time.Time {
	return d.Time
}
