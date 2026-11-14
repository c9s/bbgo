package btc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUTXOConfirmations(t *testing.T) {
	u := UTXO{Height: 100}
	assert.True(t, u.Confirmed())
	assert.Equal(t, int64(1), u.Confirmations(100))
	assert.Equal(t, int64(6), u.Confirmations(105))
	assert.Equal(t, int64(0), u.Confirmations(99), "a tip behind the output")

	mempool := UTXO{}
	assert.False(t, mempool.Confirmed())
	assert.Equal(t, int64(0), mempool.Confirmations(105))
	assert.True(t, mempool.Mature(105))
}

func TestUTXOMature(t *testing.T) {
	cb := UTXO{Height: 100, Coinbase: true}
	assert.False(t, cb.Mature(100))
	assert.False(t, cb.Mature(198))
	assert.True(t, cb.Mature(199), "spendable in block 200, 100 blocks later")

	assert.True(t, (&UTXO{Height: 100}).Mature(100))
}

func TestDescriptorAddress(t *testing.T) {
	assert.Equal(t, "bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p",
		DescriptorAddress("addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)#pqslph3d"))
	assert.Equal(t, "1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm", DescriptorAddress("addr(1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm)"))
	assert.Equal(t, "", DescriptorAddress("raw(6a24aa21)#a23t0yke"))
	assert.Equal(t, "", DescriptorAddress("wpkh([d34db33f/84h/0h/0h/0/1]02aa)#x"))
}

func TestScanUnspentUTXO(t *testing.T) {
	block := MustHash(genesisHash)
	u := ScanUnspent{
		TxID: MustHash(genesisHash), Vout: 2, ScriptPubKey: "0014aa",
		Desc:   "addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)#pqslph3d",
		Amount: 5, Coinbase: true, Height: 7, BlockHash: &block,
	}

	assert.Equal(t, UTXO{
		Outpoint: Outpoint{TxID: MustHash(genesisHash), Vout: 2},
		Address:  "bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p",
		Script:   "0014aa", Value: 5, Height: 7, BlockHash: block, Coinbase: true,
	}, u.UTXO())
}
