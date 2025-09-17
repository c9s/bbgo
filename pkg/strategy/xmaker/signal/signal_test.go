package signal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	configJson := `[
 		{ "depthRatio": { "priceRange": 0.01, "minRatio": 0.5 , "weight": 1.0 }},
		{ "bollingerBandTrend": { "interval": "1m", "window": 14, "minBandWidth": 1.0, "maxBandWidth": 2.0, "weight": 1.0 }},
		{ "liquidityDemand": { "interval": "1m", "window": 10, "threshold": 1000000, "weight": 1.0 }}
	]`

	config := DynamicConfig{}
	err := json.Unmarshal([]byte(configJson), &config)
	assert.NoError(t, err)
	assert.Len(t, config.Signals, 3)
	for _, sig := range config.Signals {
		assert.True(t, sig.Weight > 0)
	}
}
