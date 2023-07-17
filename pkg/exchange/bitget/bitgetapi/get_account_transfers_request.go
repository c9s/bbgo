package bitgetapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AccountType string

const (
	AccountExchange AccountType = "EXCHANGE"
	AccountContract AccountType = "CONTRACT"
)

type Transfer struct {
	CTime     types.MillisecondTimestamp `json:"cTime"`
	CoinId    string                     `json:"coinId"`
	CoinName  string                     `json:"coinName"`
	GroupType string                     `json:"groupType"`
	BizType   string                     `json:"bizType"`
	Quantity  fixedpoint.Value           `json:"quantity"`
	Balance   fixedpoint.Value           `json:"balance"`
	Fees      fixedpoint.Value           `json:"fees"`
	BillId    string                     `json:"billId"`
}

//go:generate GetRequest -url "/api/spot/v1/account/transferRecords" -type GetAccountTransfersRequest -responseDataType []Transfer
type GetAccountTransfersRequest struct {
	client requestgen.AuthenticatedAPIClient

	coinId   int         `param:"coinId"`
	fromType AccountType `param:"fromType"`
	after    string      `param:"after"`
	before   string      `param:"before"`
}

func (c *RestClient) NewGetAccountTransfersRequest() *GetAccountTransfersRequest {
	return &GetAccountTransfersRequest{client: c}
}
