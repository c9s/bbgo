package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseInterval(t *testing.T) {
	assert.Equal(t, ParseInterval("1s"), 1)
	assert.Equal(t, ParseInterval("3m"), 3*60)
	assert.Equal(t, ParseInterval("15h"), 15*60*60)
	assert.Equal(t, ParseInterval("72d"), 72*24*60*60)
	assert.Equal(t, ParseInterval("3Mo"), 3*30*24*60*60)
}
