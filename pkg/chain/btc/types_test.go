package btc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutpoint(t *testing.T) {
	o, err := ParseOutpoint(genesisHash + ":1")
	require.NoError(t, err)
	assert.Equal(t, MustHash(genesisHash), o.TxID)
	assert.Equal(t, uint32(1), o.Vout)
	assert.Equal(t, genesisHash+":1", o.String())

	for _, in := range []string{
		genesisHash,
		genesisHash + ":",
		genesisHash + ":-1",
		genesisHash + ":4294967296",
		"00:1",
	} {
		_, err := ParseOutpoint(in)
		assert.Error(t, err, in)
	}
}
