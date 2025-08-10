package bfxapi

import (
	"encoding/json"

	"github.com/c9s/requestgen"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PairConfig struct {
	Pair string

	p1 any // placeholder
	p2 any // placeholder
	p3 any // placeholder

	MinOrderSize fixedpoint.Value
	MaxOrderSize fixedpoint.Value

	p6 any // placeholder
	p7 any // placeholder
	p8 any // placeholder

	InitialMargin fixedpoint.Value
	MinMargin     fixedpoint.Value

	p11 any // placeholder
	p12 any // placeholder
}

// PairConfigResponse represents the config info for a trading pair from Bitfinex conf endpoint.
type PairConfigResponse struct {
	Pairs []PairConfig
}

// UnmarshalJSON parses the Bitfinex conf/pub:info:pair response format.
// Example response:
// [[PAIR, [[_, _, _, MIN_ORDER_SIZE, MAX_ORDER_SIZE, _, _, _, INITIAL_MARGIN, MIN_MARGIN], ...]]]
func (r *PairConfigResponse) UnmarshalJSON(data []byte) error {
	var raws [][][]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}

	for _, wrapper := range raws {
		for _, raw := range wrapper {
			if len(raw) != 2 || len(raw[0]) < 1 {
				return nil // no pairs found
			}

			var pair PairConfig

			symbol := raw[0]
			if len(symbol) == 0 {
				continue // skip empty pairs
			}

			pair.Pair = string(symbol[0]) // first element is the pair symbol

			if len(raw[1]) < 10 {
				logrus.Errorf("pair config for %s is incomplete, input: %s", pair, raw[1])
				continue // skip incomplete config
			}

			if err := parseJsonArray(raw[1], &pair, 1); err != nil {
				logrus.Errorf("failed to parse pair config for %v, input: %s", err, raw[1])
				return err
			}

			r.Pairs = append(r.Pairs, pair)
		}
	}

	return nil
}

// GetPairConfigRequest for Bitfinex conf endpoint.
//
//go:generate requestgen -type GetPairConfigRequest -method GET -url "/v2/conf/pub:info:pair" -responseType PairConfigResponse
type GetPairConfigRequest struct {
	client requestgen.APIClient
}

// NewGetPairConfigRequest creates a new request for pair config info.
func (c *Client) NewGetPairConfigRequest() *GetPairConfigRequest {
	return &GetPairConfigRequest{client: c}
}
