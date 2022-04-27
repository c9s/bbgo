package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type DepositHistory struct {
	Amount        fixedpoint.Value           `json:"amount"`
	Coin          string                     `json:"coin"`
	Network       string                     `json:"network"`
	Status        int                        `json:"status"`
	Address       string                     `json:"address"`
	AddressTag    string                     `json:"addressTag"`
	TxId          string                     `json:"txId"`
	InsertTime    types.MillisecondTimestamp `json:"insertTime"`
	TransferType  int                        `json:"transferType"`
	UnlockConfirm int                        `json:"unlockConfirm"`
	ConfirmTimes  string                     `json:"confirmTimes"`
}

//go:generate requestgen -method GET -url "/sapi/v1/capital/deposit/hisrec" -type GetDepositHistoryRequest -responseType []DepositHistory
type GetDepositHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	coin *string `param:"coin"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *RestClient) NewGetDepositHistoryRequest() *GetDepositHistoryRequest {
	return &GetDepositHistoryRequest{client: c}
}
