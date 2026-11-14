package btc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const genesisHash = "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"

func TestHashByteOrder(t *testing.T) {
	h, err := NewHashFromString(genesisHash)
	require.NoError(t, err)

	// the leading zeros of the displayed hash are the trailing bytes of the
	// double SHA-256: the proof of work is on the little-endian number
	assert.Equal(t, byte(0x6f), h[0])
	assert.Equal(t, byte(0x00), h[HashSize-1])
	assert.Equal(t, genesisHash, h.String())
}

func TestHashJSON(t *testing.T) {
	var v struct {
		H Hash  `json:"h"`
		P *Hash `json:"p,omitempty"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{"h":"`+genesisHash+`"}`), &v))
	assert.Equal(t, MustHash(genesisHash), v.H)
	assert.Nil(t, v.P)

	out, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"h":"`+genesisHash+`"}`, string(out))
}

func TestHashErrors(t *testing.T) {
	_, err := NewHashFromString("00")
	assert.Error(t, err)

	_, err = NewHashFromString("zz" + genesisHash[2:])
	assert.Error(t, err)

	assert.True(t, Hash{}.IsZero())
	assert.False(t, MustHash(genesisHash).IsZero())
}
