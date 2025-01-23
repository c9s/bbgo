package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type Position struct {
	Adl          fixedpoint.Value `json:"adl"`
	AvailPos     fixedpoint.Value `json:"availPos"`
	AvgPx        fixedpoint.Value `json:"avgPx"`
	BaseBal      fixedpoint.Value `json:"baseBal"`
	BaseBorrowed fixedpoint.Value `json:"baseBorrowed"`
	BaseInterest fixedpoint.Value `json:"baseInterest"`
	BePx         fixedpoint.Value `json:"bePx"`
	BizRefId     string           `json:"bizRefId"`
	BizRefType   string           `json:"bizRefType"`

	Ccy                    string           `json:"ccy"`
	ClSpotInUseAmt         string           `json:"clSpotInUseAmt"`
	CloseOrderAlgo         []interface{}    `json:"closeOrderAlgo"`
	DeltaBS                string           `json:"deltaBS"`
	DeltaPA                string           `json:"deltaPA"`
	Fee                    fixedpoint.Value `json:"fee"`
	FundingFee             fixedpoint.Value `json:"fundingFee"`
	GammaBS                string           `json:"gammaBS"`
	GammaPA                string           `json:"gammaPA"`
	IdxPx                  fixedpoint.Value `json:"idxPx"`
	Imr                    string           `json:"imr"`
	InstId                 string           `json:"instId"`
	InstType               string           `json:"instType"`
	Interest               fixedpoint.Value `json:"interest"`
	Last                   fixedpoint.Value `json:"last"`
	Lever                  fixedpoint.Value `json:"lever"`
	Liab                   fixedpoint.Value `json:"liab"`
	LiabCcy                string           `json:"liabCcy"`
	LiqPenalty             string           `json:"liqPenalty"`
	LiqPx                  fixedpoint.Value `json:"liqPx"`
	Margin                 fixedpoint.Value `json:"margin"`
	MarkPx                 fixedpoint.Value `json:"markPx"`
	MaxSpotInUseAmt        string           `json:"maxSpotInUseAmt"`
	MgnMode                string           `json:"mgnMode"`
	MgnRatio               fixedpoint.Value `json:"mgnRatio"`
	Mmr                    string           `json:"mmr"`
	NotionalUsd            string           `json:"notionalUsd"`
	OptVal                 string           `json:"optVal"`
	PendingCloseOrdLiabVal string           `json:"pendingCloseOrdLiabVal"`
	Pnl                    fixedpoint.Value `json:"pnl"`
	Pos                    fixedpoint.Value `json:"pos"`
	PosCcy                 string           `json:"posCcy"`
	PosId                  string           `json:"posId"`
	PosSide                string           `json:"posSide"`
	QuoteBal               fixedpoint.Value `json:"quoteBal"`
	QuoteBorrowed          fixedpoint.Value `json:"quoteBorrowed"`
	QuoteInterest          fixedpoint.Value `json:"quoteInterest"`
	RealizedPnl            fixedpoint.Value `json:"realizedPnl"`
	SpotInUseAmt           string           `json:"spotInUseAmt"`
	SpotInUseCcy           string           `json:"spotInUseCcy"`
	ThetaBS                string           `json:"thetaBS"`
	ThetaPA                string           `json:"thetaPA"`
	TradeId                string           `json:"tradeId"`

	CreationTime types.MillisecondTimestamp `json:"cTime"`
	UpdatedTime  types.MillisecondTimestamp `json:"uTime"`

	Upl            string           `json:"upl"`
	UplLastPx      fixedpoint.Value `json:"uplLastPx"`
	UplRatio       string           `json:"uplRatio"`
	UplRatioLastPx string           `json:"uplRatioLastPx"`
	UsdPx          fixedpoint.Value `json:"usdPx"`
	VegaBS         string           `json:"vegaBS"`
	VegaPA         string           `json:"vegaPA"`
}

//go:generate GetRequest -url "/api/v5/account/positions" -type GetAccountPositionsRequest -responseDataType []Position
type GetAccountPositionsRequest struct {
	client requestgen.AuthenticatedAPIClient

	instType *InstrumentType `param:"instType"`
}

func (c *RestClient) NewGetAccountPositionsRequest() *GetAccountPositionsRequest {
	return &GetAccountPositionsRequest{
		client: c,
	}
}
