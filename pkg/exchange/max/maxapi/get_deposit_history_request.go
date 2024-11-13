package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type DepositState string

const (
	DepositStateSubmitting DepositState = "submitting"
	DepositStateCancelled  DepositState = "cancelled"
	DepositStateSubmitted  DepositState = "submitted"
	DepositStatePending    DepositState = "pending"
	DepositStateSuspect    DepositState = "suspect"
	DepositStateRejected   DepositState = "rejected"
	DepositStateSuspended  DepositState = "suspended"
	DepositStateAccepted   DepositState = "accepted"
	DepositStateChecking   DepositState = "checking"

	// v3 states
	DepositStateProcessing DepositState = "processing"
	DepositStateFailed     DepositState = "failed"
	DepositStateDone       DepositState = "done"
)

type Deposit struct {
	Currency        string                     `json:"currency"`         // "eth"
	NetworkProtocol string                     `json:"network_protocol"` // "ethereum-erc20"
	Amount          fixedpoint.Value           `json:"amount"`
	Fee             fixedpoint.Value           `json:"fee"`
	TxID            string                     `json:"txid"`
	State           DepositState               `json:"state"`
	StateReason     string                     `json:"state_reason"`
	Status          string                     `json:"status"`
	Confirmations   int64                      `json:"confirmations"`
	Address         string                     `json:"to_address"` // 0x5c7d23d516f120d322fc7b116386b7e491739138
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	UpdatedAt       types.MillisecondTimestamp `json:"updated_at"`
}

//go:generate GetRequest -url "v3/deposits" -type GetDepositHistoryRequest -responseType []Deposit
type GetDepositHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  *string    `param:"currency"`
	timestamp *time.Time `param:"timestamp,milliseconds"` // seconds
	state     *string    `param:"state"`                  // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect

	order *string `param:"order"`

	limit *int `param:"limit"`
}

func (c *RestClient) NewGetDepositHistoryRequest() *GetDepositHistoryRequest {
	return &GetDepositHistoryRequest{
		client: c,
	}
}
