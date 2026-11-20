package evm

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHeader(number int64) *ethtypes.Header {
	return &ethtypes.Header{
		ParentHash: common.HexToHash("0x01"),
		Number:     big.NewInt(number),
		Difficulty: big.NewInt(0),
		GasLimit:   30_000_000,
		Time:       1_700_000_000,
	}
}

// rpcBlockJSON renders a block the way a node answers eth_getBlockByNumber.
func rpcBlockJSON(t *testing.T, header *ethtypes.Header, hash common.Hash, txs []json.RawMessage) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(header)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	fields["hash"], err = json.Marshal(hash)
	require.NoError(t, err)

	if txs == nil {
		txs = []json.RawMessage{}
	}

	fields["transactions"], err = json.Marshal(txs)
	require.NoError(t, err)

	out, err := json.Marshal(fields)
	require.NoError(t, err)
	return out
}

func signedTxJSON(t *testing.T, value int64) (json.RawMessage, common.Hash, common.Address) {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     7,
		GasTipCap: big.NewInt(1e9),
		GasFeeCap: big.NewInt(30e9),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(value),
	})

	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(big.NewInt(1)), key)
	require.NoError(t, err)

	raw, err := signed.MarshalJSON()
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(key.PublicKey)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	fields["from"], _ = json.Marshal(from)

	out, err := json.Marshal(fields)
	require.NoError(t, err)

	return out, signed.Hash(), from
}

func TestDecodeBlock(t *testing.T) {
	t.Run("full transactions keep the sender and skip unknown types", func(t *testing.T) {
		txJSON, txHash, from := signedTxJSON(t, 5e18)
		unknown := json.RawMessage(`{"type":"0x7f","hash":"0x02"}`)

		// a hash that differs from the recomputed one must be kept as reported
		reported := common.HexToHash("0xabcdef")
		raw := rpcBlockJSON(t, testHeader(100), reported, []json.RawMessage{txJSON, unknown})

		block, err := DecodeBlock(raw, true)
		require.NoError(t, err)

		assert.Equal(t, reported, block.Hash)
		assert.Equal(t, uint64(100), block.Number())
		assert.Equal(t, common.HexToHash("0x01"), block.ParentHash())
		require.Len(t, block.Transactions, 1)
		assert.Equal(t, txHash, block.Transactions[0].Hash())
		assert.Equal(t, from, block.Transactions[0].From)
		assert.Equal(t, big.NewInt(5e18), block.Transactions[0].Value())
		assert.Equal(t, 1, block.SkippedTransactions)
		assert.Equal(t, 2, block.TxCount())
	})

	t.Run("transaction hashes only", func(t *testing.T) {
		h1, h2 := common.HexToHash("0x11"), common.HexToHash("0x22")
		hashes := []json.RawMessage{
			json.RawMessage(`"` + h1.Hex() + `"`),
			json.RawMessage(`"` + h2.Hex() + `"`),
		}

		block, err := DecodeBlock(rpcBlockJSON(t, testHeader(1), common.HexToHash("0x99"), hashes), false)
		require.NoError(t, err)
		assert.Equal(t, []common.Hash{h1, h2}, block.TxHashes)
		assert.Nil(t, block.Transactions)
		assert.Equal(t, 2, block.TxCount())
	})

	t.Run("null is not found", func(t *testing.T) {
		_, err := DecodeBlock(json.RawMessage("null"), false)
		assert.ErrorIs(t, err, ethereum.NotFound)
	})

	t.Run("a pending block without hash is rejected", func(t *testing.T) {
		raw, err := json.Marshal(testHeader(1))
		require.NoError(t, err)

		// go-ethereum renders the computed hash; a pending block has none
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		delete(fields, "hash")
		raw, err = json.Marshal(fields)
		require.NoError(t, err)

		_, err = DecodeBlock(raw, false)
		assert.Error(t, err)
	})
}

func TestToBlockNumArg(t *testing.T) {
	assert.Equal(t, "latest", toBlockNumArg(nil))
	assert.Equal(t, "0x10", toBlockNumArg(big.NewInt(16)))
	assert.Equal(t, "finalized", toBlockNumArg(big.NewInt(-3)))
}
