package hyperapi

import (
	"encoding/hex"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// import msgpack

// # Create order_wire dict matching the Go test case
// # Python dict preserves insertion order (Python 3.7+)
// # Order: a, b, p, s, r, t (and optionally c)
// order_wire = {
//     "a": 0,
//     "b": True,
//     "p": "100000",
//     "s": "0.001",
//     "r": False,
//     "t": {
//         "limit": {
//             "tif": "Gtc"
//         }
//     }
// }

// # Serialize with msgpack
// # Use use_bin_type=False to match the behavior expected by Go
// packed = msgpack.packb(order_wire, use_bin_type=False)

// # Convert to hex string
// pythonExpectedHex = packed.hex()
func Test_Msgpack_OrderAction(t *testing.T) {
	orderTypeNew := OrderWireType{
		Limit: &OrderWireTypeLimit{
			Tif: TimeInForceGTC,
		},
	}

	newOrder := OrderWire{
		Asset:      0,
		IsBuy:      true,
		LimitPx:    "100000",
		Size:       "0.001",
		ReduceOnly: false,
		OrderType:  orderTypeNew,
		Cloid:      nil, // No cloid for this test
	}

	// Serialize with msgpack
	newBytes, err := msgpack.Marshal(newOrder)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	goHex := hex.EncodeToString(newBytes)

	// Expected output from Python SDK for equivalent order_wire
	// Python code: {"a": 0, "b": True, "p": "100000", "s": "0.001", "r": False, "t": {"limit": {"tif": "Gtc"}}}
	pythonExpectedHex := "86a16100a162c3a170a6313030303030a173a5302e303031a172c2a17481a56c696d697481a3746966a3477463"

	t.Logf("Go msgpack output:     %s", goHex)
	t.Logf("Python expected:       %s", pythonExpectedHex)

	if goHex != pythonExpectedHex {
		t.Errorf(
			"Msgpack output does NOT match Python SDK!\nGot:      %s\nExpected: %s",
			goHex,
			pythonExpectedHex,
		)
	} else {
		t.Logf("✓ Msgpack field ordering matches Python SDK!")
	}
}
