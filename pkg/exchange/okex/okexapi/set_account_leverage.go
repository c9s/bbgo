package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type MarginMode string

const (
	MarginModeIsolated = "isolated"
	MarginModeCross    = "cross"
)

type LeverageResponse struct {
	Leverage fixedpoint.Value `json:"lever"`

	MarginMode MarginMode `json:"mgnMode"`
	InstId     string     `json:"instId"`
	PosSide    string     `json:"posSide"`
}

//go:generate PostRequest -url "/api/v5/account/set-leverage" -type SetAccountLeverageRequest -responseDataType []LeverageResponse
type SetAccountLeverageRequest struct {
	client requestgen.AuthenticatedAPIClient

	leverage   int        `param:"lever"`
	marginMode MarginMode `param:"mgnMode"`

	// instrumentId: Under cross mode, either instId or ccy is required; if both are passed, instId will be used by default.
	instrumentId *string `param:"instId"`

	// currency: Currency used for margin, used for the leverage setting for the currency in auto borrow.
	// Only applicable to cross MARGIN of Spot mode/Multi-currency margin/Portfolio margin
	// Required when setting the leverage for automatically borrowing coin.
	currency *string `param:"ccy"`

	posSide *string `param:"posSide"`
}

func (c *RestClient) NewSetAccountLeverageRequest() *SetAccountLeverageRequest {
	return &SetAccountLeverageRequest{
		client: c,
	}
}
