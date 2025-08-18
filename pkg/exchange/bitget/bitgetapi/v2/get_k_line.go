package bitgetapi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type KLine struct {
	// System timestamp, Unix millisecond timestamp, e.g. 1690196141868
	Ts    types.MillisecondTimestamp
	Open  fixedpoint.Value
	High  fixedpoint.Value
	Low   fixedpoint.Value
	Close fixedpoint.Value
	// Trading volume in base currency, e.g. "BTC" in the "BTCUSD" pair.
	Volume fixedpoint.Value
	// Trading volume in quote currency, e.g. "USD" in the "BTCUSD" pair.
	QuoteVolume fixedpoint.Value
	// Trading volume in USDT
	UsdtVolume fixedpoint.Value
}

type KLineResponse []KLine

const KLinesArrayLen = 8

func (k *KLine) UnmarshalJSON(data []byte) error {
	var jsonArr []json.RawMessage
	err := json.Unmarshal(data, &jsonArr)
	if err != nil {
		return fmt.Errorf("failed to unmarshal jsonRawMessage: %v, err: %w", string(data), err)
	}
	if len(jsonArr) != KLinesArrayLen {
		return fmt.Errorf("unexpected K Lines array length: %d, exp: %d", len(jsonArr), KLinesArrayLen)
	}

	err = json.Unmarshal(jsonArr[0], &k.Ts)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resp index 0: %v, err: %w", string(jsonArr[0]), err)
	}

	values := make([]fixedpoint.Value, len(jsonArr)-1)
	for i, jsonRaw := range jsonArr[1:] {
		err = json.Unmarshal(jsonRaw, &values[i])
		if err != nil {
			return fmt.Errorf("failed to unmarshal resp index %d: %v, err: %w", i+1, string(jsonRaw), err)
		}
	}
	k.Open = values[0]
	k.High = values[1]
	k.Low = values[2]
	k.Close = values[3]
	k.Volume = values[4]
	k.QuoteVolume = values[5]
	k.UsdtVolume = values[6]

	return nil
}

//go:generate GetRequest -url "/api/v2/spot/market/candles" -type GetKLineRequest -responseDataType .KLineResponse
type GetKLineRequest struct {
	client requestgen.APIClient

	symbol      string     `param:"symbol,query"`
	granularity string     `param:"granularity,query"`
	startTime   *time.Time `param:"startTime,milliseconds,query"`
	endTime     *time.Time `param:"endTime,milliseconds,query"`
	// Limit number default 100 max 1000
	limit *string `param:"limit,query"`
}

func (s *Client) NewGetKLineRequest() *GetKLineRequest {
	return &GetKLineRequest{client: s.Client}
}
