package cmd

import (
	"testing"

	asserts "github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

func TestCoinbaseTag(t *testing.T) {
	// the coinbase field of mainnet block 975765
	tag := coinbaseTag("0395e30e2cfabe6d6d20ab6e5e35e0e26f60e204a70d413eefd496e3f3ed3ac83725025b5e28c89a9010000000f09f909f092f4632506f6f6c2f61000000000000000000000000000000000000000000000000000000000000000000000005002b000000")
	asserts.Equal(t, "/F2Pool/a", tag)

	asserts.Equal(t, "", coinbaseTag("03"))
	asserts.Equal(t, "", coinbaseTag("zz"))
}

func TestScriptLabel(t *testing.T) {
	asserts.Equal(t, "bc1qyz5ts3dl4wxntavwfkugsj25zaaj8hcwmjkl6h", scriptLabel(&btc.ScriptPubKey{
		Address: "bc1qyz5ts3dl4wxntavwfkugsj25zaaj8hcwmjkl6h", Type: btc.ScriptTypeWitnessV0KeyHash,
	}))

	asserts.Equal(t, "OP_RETURN 455853", scriptLabel(&btc.ScriptPubKey{
		Asm: "OP_RETURN 455853", Type: btc.ScriptTypeNullData,
	}))

	asserts.Equal(t, "(multisig)", scriptLabel(&btc.ScriptPubKey{Type: btc.ScriptTypeMultisig}))
}

func TestDescriptorAddress(t *testing.T) {
	asserts.Equal(t, "bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p",
		descriptorAddress("addr(bc1qh2aq97wafs7px4jnajnpleu0njaqmque8dku3p)#pqslph3d"))
	asserts.Equal(t, "1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm", descriptorAddress("addr(1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm)"))
	asserts.Equal(t, "", descriptorAddress("raw(6a24aa21)#a23t0yke"))
	asserts.Equal(t, "", descriptorAddress("wpkh([d34db33f/84h/0h/0h/0/1]02aa)#x"))
}
