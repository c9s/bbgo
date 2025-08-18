package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// 1 for internal transfer, 0 for external transfer
//
//go:generate stringer -type=TransferType
type TransferType int

const (
	TransferTypeInternal TransferType = 0
	TransferTypeExternal TransferType = 0
)

type WithdrawRecord struct {
	Id              string           `json:"id"`
	Address         string           `json:"address"`
	Amount          fixedpoint.Value `json:"amount"`
	ApplyTime       string           `json:"applyTime"`
	Coin            string           `json:"coin"`
	WithdrawOrderID string           `json:"withdrawOrderId"`
	Network         string           `json:"network"`
	TransferType    TransferType     `json:"transferType"`
	Status          WithdrawStatus   `json:"status"`
	TransactionFee  fixedpoint.Value `json:"transactionFee"`
	ConfirmNo       int              `json:"confirmNo"`
	Info            string           `json:"info"`
	TxID            string           `json:"txId"`
}

//go:generate stringer -type=WithdrawStatus -trimprefix=WithdrawStatus
type WithdrawStatus int

// WithdrawStatus: 0(0:Email Sent,1:Cancelled 2:Awaiting Approval 3:Rejected 4:Processing 5:Failure 6:Completed)
const (
	WithdrawStatusEmailSent WithdrawStatus = iota
	WithdrawStatusCancelled
	WithdrawStatusAwaitingApproval
	WithdrawStatusRejected
	WithdrawStatusProcessing
	WithdrawStatusFailure
	WithdrawStatusCompleted
)

//go:generate requestgen -method GET -url "/sapi/v1/capital/withdraw/history" -type GetWithdrawHistoryRequest -responseType []WithdrawRecord
type GetWithdrawHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient
	coin   string `param:"coin"`

	withdrawOrderId *string `param:"withdrawOrderId"`

	status *WithdrawStatus `param:"status"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *uint64    `param:"limit"`
	offset    *uint64    `param:"offset"`
}

func (c *RestClient) NewGetWithdrawHistoryRequest() *GetWithdrawHistoryRequest {
	return &GetWithdrawHistoryRequest{client: c}
}
