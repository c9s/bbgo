package hyperapi

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/requestgen"
)

type FuturesMetaAndAssetCtxsResponse struct {
	Meta      FuturesGetMetaResponse
	AssetCtxs []FuturesAssetContext
}

type FuturesAssetContext struct {
	DayNotionalVolume string    `json:"dayNtlVlm"`
	Funding           string    `json:"funding"`
	ImpactPrices      [2]string `json:"impactPxs"`
	MarkPrice         string    `json:"markPx"`
	MidPrice          string    `json:"midPx"`
	OpenInterest      string    `json:"openInterest"`
	OraclePrice       string    `json:"oraclePx"`
	Premium           string    `json:"premium"`
	PrevDayPrice      string    `json:"prevDayPx"`
}

func (r *FuturesMetaAndAssetCtxsResponse) UnmarshalJSON(data []byte) error {
	var payload []json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if len(payload) != 2 {
		return fmt.Errorf("unexpected metaAndAssetCtxs payload length %d", len(payload))
	}

	if err := json.Unmarshal(payload[0], &r.Meta); err != nil {
		return fmt.Errorf("unmarshal meta block: %w", err)
	}

	if err := json.Unmarshal(payload[1], &r.AssetCtxs); err != nil {
		return fmt.Errorf("unmarshal asset contexts: %w", err)
	}

	return nil
}

//go:generate requestgen -method POST -url "/info" -type FuturesGetMetaAndAssetCtxsRequest -responseType FuturesMetaAndAssetCtxsResponse

type FuturesGetMetaAndAssetCtxsRequest struct {
	client requestgen.APIClient

	metaType ReqTypeInfo `param:"type" default:"metaAndAssetCtxs" validValues:"metaAndAssetCtxs"`
}

func (c *Client) NewFuturesGetMetaAndAssetCtxsRequest() *FuturesGetMetaAndAssetCtxsRequest {
	return &FuturesGetMetaAndAssetCtxsRequest{
		client:   c,
		metaType: ReqFuturesMetaAndAssetCtxs,
	}
}
