package v3

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Client) NewGetMarginLiquidationHistoryRequest() *GetMarginLiquidationHistoryRequest {
	return &GetMarginLiquidationHistoryRequest{client: s.Client}
}

type LiquidationRecord struct {
	SN              string                     `json:"sn"`
	AdRatio         fixedpoint.Value           `json:"ad_ratio"`
	ExpectedAdRatio fixedpoint.Value           `json:"expected_ad_ratio"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	State           LiquidationState           `json:"state"`
}

type LiquidationState string

const (
	LiquidationStateProcessing LiquidationState = "processing"
	LiquidationStateDebt       LiquidationState = "debt"
	LiquidationStateLiquidated LiquidationState = "liquidated"
)

//go:generate GetRequest -url "/api/v3/wallet/m/liquidations" -type GetMarginLiquidationHistoryRequest -responseType []LiquidationRecord
type GetMarginLiquidationHistoryRequest struct {
	client    requestgen.AuthenticatedAPIClient
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
	limit     *int       `param:"limit"`
}
