package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type DepositStatus string

const (
	// EMPTY string means not supported

	DepositPending = DepositStatus("pending")

	DepositRejected = DepositStatus("rejected")

	DepositSuccess = DepositStatus("success")

	DepositCancelled = DepositStatus("canceled")

	// created but can not withdraw
	DepositCredited = DepositStatus("credited")
)

type Deposit struct {
	GID           int64            `json:"gid" db:"gid"`
	Exchange      ExchangeName     `json:"exchange" db:"exchange"`
	Time          Time             `json:"time" db:"time"`
	Amount        fixedpoint.Value `json:"amount" db:"amount"`
	Asset         string           `json:"asset" db:"asset"`
	Address       string           `json:"address" db:"address"`
	AddressTag    string           `json:"addressTag"`
	TransactionID string           `json:"transactionID" db:"txn_id"`
	Status        DepositStatus    `json:"status"`

	// Required confirm for unlock balance
	UnlockConfirm int `json:"unlockConfirm"`

	// Confirmation format = "current/required", for example: "7/16"
	Confirmation string `json:"confirmation"`
}

func (d Deposit) GetCurrentConfirmation() (current int, required int) {
	if len(d.Confirmation) == 0 {
		return 0, 0
	}

	strs := strings.Split(d.Confirmation, "/")
	if len(strs) < 2 {
		return 0, 0
	}

	current, _ = strconv.Atoi(strs[0])
	required, _ = strconv.Atoi(strs[1])
	return current, required
}

func (d Deposit) EffectiveTime() time.Time {
	return d.Time.Time()
}

func (d Deposit) String() (o string) {
	o = fmt.Sprintf("%s deposit %s %v <- ", d.Exchange, d.Asset, d.Amount)

	if len(d.AddressTag) > 0 {
		o += fmt.Sprintf("%s (tag: %s) at %s", d.Address, d.AddressTag, d.Time.Time())
	} else {
		o += fmt.Sprintf("%s at %s", d.Address, d.Time.Time())
	}

	if len(d.TransactionID) > 0 {
		o += fmt.Sprintf("txID: %s", cutstr(d.TransactionID, 12, 4, 4))
	}
	if len(d.Status) > 0 {
		o += "status: " + string(d.Status)
	}

	return o
}
