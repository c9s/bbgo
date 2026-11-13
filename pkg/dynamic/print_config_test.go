package dynamic

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type PrintConfigConfig struct {
	Margin    fixedpoint.Value `json:"margin"`
	MakerOnly bool             `json:"makerOnly"`
}

type printConfigStrategy struct {
	PrintConfigConfig

	Symbol string `json:"symbol"`
}

func (s *printConfigStrategy) ID() string { return "printtest" }

func TestPrintConfig_EmbeddedAnonymousStruct(t *testing.T) {
	s := &printConfigStrategy{
		PrintConfigConfig: PrintConfigConfig{
			Margin:    fixedpoint.NewFromFloat(0.0015),
			MakerOnly: true,
		},
		Symbol: "BTCUSDT",
	}

	var buf bytes.Buffer
	// style=nil takes the plain-text "json: value" writer path.
	PrintConfig(s, &buf, nil, false, DefaultWhiteList()...)

	out := buf.String()
	// top-level json field
	assert.Contains(t, out, "symbol: ", "top-level field should be printed")
	// promoted fields from the embedded anonymous config struct
	assert.Contains(t, out, "margin: ", "embedded config field should be printed")
	assert.Contains(t, out, "makerOnly: ", "embedded config field should be printed")
}
