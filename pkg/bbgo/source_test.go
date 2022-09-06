package bbgo

import (
	"encoding/json"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestSource(t *testing.T) {
	input := "{\"source\":\"high\"}"
	type Strategy struct {
		SourceSelector
	}
	s := Strategy{}
	assert.NoError(t, json.Unmarshal([]byte(input), &s))
	assert.Equal(t, s.Source.Source, "high")
	assert.NotNil(t, s.Source.sourceGetter)
	e, err := json.Marshal(&s)
	assert.NoError(t, err)
	assert.Equal(t, input, string(e))

	input = "{}"
	s = Strategy{}
	assert.NoError(t, json.Unmarshal([]byte(input), &s))
	assert.Equal(t, fixedpoint.Zero, s.GetSource(&types.KLine{}))

	e, err = json.Marshal(&Strategy{})
	assert.NoError(t, err)
	assert.Equal(t, "{\"source\":\"close\"}", string(e))

}
