package hyperapi

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/requestgen"
)

type SpotMetaAndAssetCtxsResponse struct {
	Meta      SpotGetMetaResponse
	AssetCtxs []SpotAssetContext
}

type SpotAssetContext struct {
	DayNotionalVolume string `json:"dayNtlVlm"`
	MarkPrice         string `json:"markPx"`
	MidPrice          string `json:"midPx"`
	PrevDayPrice      string `json:"prevDayPx"`
}

func (r *SpotMetaAndAssetCtxsResponse) UnmarshalJSON(data []byte) error {
	var payload []json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if len(payload) != 2 {
		return fmt.Errorf("unexpected spotMetaAndAssetCtxs payload length %d", len(payload))
	}

	if err := json.Unmarshal(payload[0], &r.Meta); err != nil {
		return fmt.Errorf("unmarshal meta block: %w", err)
	}

	if err := json.Unmarshal(payload[1], &r.AssetCtxs); err != nil {
		return fmt.Errorf("unmarshal asset contexts: %w", err)
	}

	return nil
}

//go:generate requestgen -method POST -url "/info" -type SpotGetMetaAndAssetCtxsRequest -responseType SpotMetaAndAssetCtxsResponse

type SpotGetMetaAndAssetCtxsRequest struct {
	client requestgen.APIClient

	metaType ReqTypeInfo `param:"type" default:"spotMetaAndAssetCtxs" validValues:"spotMetaAndAssetCtxs"`
}

func (c *Client) NewSpotGetMetaAndAssetCtxsRequest() *SpotGetMetaAndAssetCtxsRequest {
	return &SpotGetMetaAndAssetCtxsRequest{
		client:   c,
		metaType: ReqSpotMetaAndAssetCtxs,
	}
}
