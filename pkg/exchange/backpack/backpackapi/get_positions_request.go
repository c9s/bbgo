package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET

// FuturePosition is an open futures position.
type FuturePosition struct {
	Symbol     string `json:"symbol"`
	PositionId string `json:"positionId"`

	UserId       int64  `json:"userId"`
	SubaccountId uint16 `json:"subaccountId,omitempty"`

	// NetQuantity is positive for a long position and negative for a short position.
	NetQuantity fixedpoint.Value `json:"netQuantity"`
	NetCost     fixedpoint.Value `json:"netCost"`

	NetExposureQuantity fixedpoint.Value `json:"netExposureQuantity"`
	NetExposureNotional fixedpoint.Value `json:"netExposureNotional"`

	EntryPrice          fixedpoint.Value `json:"entryPrice"`
	BreakEvenPrice      fixedpoint.Value `json:"breakEvenPrice"`
	EstLiquidationPrice fixedpoint.Value `json:"estLiquidationPrice"`
	MarkPrice           fixedpoint.Value `json:"markPrice"`

	Imf         fixedpoint.Value `json:"imf"`
	Mmf         fixedpoint.Value `json:"mmf"`
	ImfFunction *MarginFunction  `json:"imfFunction,omitempty"`
	MmfFunction *MarginFunction  `json:"mmfFunction,omitempty"`

	PnlRealized   fixedpoint.Value `json:"pnlRealized"`
	PnlUnrealized fixedpoint.Value `json:"pnlUnrealized"`

	CumulativeFundingPayment fixedpoint.Value `json:"cumulativeFundingPayment"`
	CumulativeInterest       fixedpoint.Value `json:"cumulativeInterest"`
}

func (p FuturePosition) IsLong() bool {
	return p.NetQuantity.Sign() > 0
}

func (p FuturePosition) IsShort() bool {
	return p.NetQuantity.Sign() < 0
}

//go:generate GetRequest -url "/api/v1/position" -type GetPositionsRequest -responseType []FuturePosition
type GetPositionsRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol     *string     `param:"symbol"`
	marketType *MarketType `param:"marketType"`
}

func (c *RestClient) NewGetPositionsRequest() *GetPositionsRequest {
	return &GetPositionsRequest{client: c.withInstruction(InstructionPositionQuery)}
}
