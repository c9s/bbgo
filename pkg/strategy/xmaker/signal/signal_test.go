package signal

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	configJson := `[
 		{ "depthRatio": { "priceRange": 0.01, "minRatio": 0.5 }, "weight": 1.0 }
	]`

	config := DynamicConfig{}
	err := json.Unmarshal([]byte(configJson), &config)
	assert.NoError(t, err)
	assert.Len(t, config.Signals, 1)
}
