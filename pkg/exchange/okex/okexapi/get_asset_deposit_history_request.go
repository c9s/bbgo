package okexapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type DepositRecord struct {
	ActualDepBlkConfirm types.StrInt64   `json:"actualDepBlkConfirm"`
	Amount              fixedpoint.Value `json:"amt"`
	AreaCodeFrom        string           `json:"areaCodeFrom"`
	Currency            string           `json:"ccy"`
	Chain               string           `json:"chain"`
	DepId               string           `json:"depId"`
	From                string           `json:"from"`
	FromWdId            string           `json:"fromWdId"`
	State               types.StrInt64   `json:"state"`
	To                  string           `json:"to"`

	Ts types.MillisecondTimestamp `json:"ts"`

	TxId string `json:"txId"`
}

//go:generate GetRequest -url "/api/v5/asset/deposit-history" -type GetAssetDepositHistoryRequest -responseDataType []DepositRecord
type GetAssetDepositHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency *string    `param:"ccy"`
	after    *time.Time `param:"after,milliseconds"`
	before   *time.Time `param:"before,milliseconds"`
	limit    *uint64    `param:"limit" defaultValue:"100"`
}

func (c *RestClient) NewGetAssetDepositHistoryRequest() *GetAssetDepositHistoryRequest {
	return &GetAssetDepositHistoryRequest{
		client: c,
	}
}
