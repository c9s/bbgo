package v3

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type DepositState string

const (
	DepositStateProcessing DepositState = "processing"
	DepositStateFailed     DepositState = "failed"
	DepositStateCanceled   DepositState = "canceled"
	DepositStateDone       DepositState = "done"
)

type Deposit struct {
	UUID            string                     `json:"uuid"`
	Currency        string                     `json:"currency"`
	NetworkProtocol string                     `json:"network_protocol"`
	Amount          fixedpoint.Value           `json:"amount"`
	Address         string                     `json:"to_address"`
	TxID            string                     `json:"txid"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	Confirmations   int64                      `json:"confirmations"`
	State           DepositState               `json:"state"`
	StateReason     string                     `json:"state_reason"`
}

//go:generate requestgen -method GET -url "/api/v3/deposits" -type GetDepositHistoryRequest -responseType []Deposit
type GetDepositHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  *string    `param:"currency"`
	timestamp *time.Time `param:"timestamp,milliseconds"`

	order *string `param:"order"`
	limit *int    `param:"limit"`
}

func (c *Client) NewGetDepositHistoryRequest() *GetDepositHistoryRequest {
	return &GetDepositHistoryRequest{
		client: c,
	}
}
