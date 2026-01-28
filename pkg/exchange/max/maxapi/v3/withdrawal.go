package v3

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

type WithdrawState string

const (
	WithdrawStateProcessing WithdrawState = "processing"
	WithdrawStateFailed     WithdrawState = "failed"
	WithdrawStateCanceled   WithdrawState = "canceled"
	WithdrawStateDone       WithdrawState = "done"
)

type TransactionType string

const (
	TransactionTypeExternal TransactionType = "external"
	TransactionTypeInternal TransactionType = "internal"
)

type Withdrawal struct {
	UUID            string                     `json:"uuid"`
	Currency        string                     `json:"currency"`
	NetworkProtocol string                     `json:"network_protocol"`
	Amount          fixedpoint.Value           `json:"amount"`
	Fee             fixedpoint.Value           `json:"fee"`
	FeeCurrency     string                     `json:"fee_currency"`
	Address         string                     `json:"to_address"`
	Label           string                     `json:"label"`
	TxID            string                     `json:"txid"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	State           WithdrawState              `json:"state"`
	TransactionType TransactionType            `json:"transaction_type"`
}

type WithdrawalAddress struct {
	UUID            string                     `json:"uuid"`
	Currency        string                     `json:"currency"`
	NetworkProtocol string                     `json:"network_protocol"`
	Address         string                     `json:"address"`
	ExtraLabel      string                     `json:"extra_label"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	ActivatedAt     types.MillisecondTimestamp `json:"activated_at"`
	IsInternal      bool                       `json:"is_internal"`
}

//go:generate requestgen -method GET -url "/api/v3/withdraw_addresses" -type GetWithdrawalAddressesRequest -responseType []WithdrawalAddress
type GetWithdrawalAddressesRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,required"`
	limit    *int   `param:"limit"`
	offset   *int   `param:"offset"`
}

//go:generate requestgen -method POST -url "/api/v3/withdrawal" -type WithdrawalRequest -responseType .Withdrawal
type WithdrawalRequest struct {
	client requestgen.AuthenticatedAPIClient

	addressUUID string  `param:"withdraw_address_uuid,required"`
	currency    string  `param:"currency,required"`
	amount      float64 `param:"amount"`
}

//go:generate requestgen -method GET -url "/api/v3/withdrawals" -type GetWithdrawalHistoryRequest -responseType []Withdrawal
type GetWithdrawalHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  *string        `param:"currency"`
	state     *WithdrawState `param:"state"`
	timestamp *time.Time     `param:"timestamp,milliseconds"` // milli-seconds

	// order could be desc or asc
	order *string `param:"order"`
	// limit's default = 50
	limit *int `param:"limit"`
}

func (c *Client) NewGetWithdrawalAddressesRequest() *GetWithdrawalAddressesRequest {
	return &GetWithdrawalAddressesRequest{
		client: c,
	}
}

func (c *Client) NewWithdrawalRequest() *WithdrawalRequest {
	return &WithdrawalRequest{client: c}
}

func (c *Client) NewGetWithdrawalHistoryRequest() *GetWithdrawalHistoryRequest {
	return &GetWithdrawalHistoryRequest{
		client: c,
	}
}
