package binance

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var jsCommentTrimmer = regexp.MustCompile("(?m)//.*$")

func TestMarginResponseParsing(t *testing.T) {
	type testcase struct {
		input string
	}

	var testcases = []testcase{
		{
			input: `{
			  "e": "executionReport",
			  "E": 1608545107403,
			  "s": "BNBUSDT",
			  "c": "ios_0de017ca7ceb4102b3baa664feb46e65",
			  "S": "BUY",
			  "o": "MARKET",
			  "f": "GTC",
			  "q": "0.50000000",
			  "p": "0.00000000",
			  "P": "0.00000000",
			  "F": "0.00000000",
			  "g": -1,
			  "C": "",
			  "x": "NEW",
			  "X": "NEW",
			  "r": "NONE",
			  "i": 1147335332,
			  "l": "0.00000000",
			  "z": "0.00000000",
			  "L": "0.00000000",
			  "n": "0",
			  "N": null,
			  "T": 1608545107401,
			  "t": -1,
			  "I": 2387303818,
			  "w": true,
			  "m": false,
			  "M": false,
			  "O": 1608545107401,
			  "Z": "0.00000000",
			  "Y": "0.00000000",
			  "Q": "0.00000000"
			}`,
		},
		{
			input: `{
			  "e": "executionReport",
			  "E": 1608545107403,
			  "s": "BNBUSDT",
			  "c": "ios_0de017ca7ceb4102b3baa664feb46e65",
			  "S": "BUY",
			  "o": "MARKET",
			  "f": "GTC",
			  "q": "0.50000000",
			  "p": "0.00000000",
			  "P": "0.00000000",
			  "F": "0.00000000",
			  "g": -1,
			  "C": "",
			  "x": "TRADE",
			  "X": "FILLED",
			  "r": "NONE",
			  "i": 1147335332,
			  "l": "0.50000000",
			  "z": "0.50000000",
			  "L": "33.85710000",
			  "n": "0.00037500",
			  "N": "BNB",
			  "T": 1608545107401,
			  "t": 98414801,
			  "I": 2387303819,
			  "w": false,
			  "m": false,
			  "M": true,
			  "O": 1608545107401,
			  "Z": "16.92855000",
			  "Y": "16.92855000",
			  "Q": "0.00000000"
			}`,
		},
		{
			input: `{
			  "e": "outboundAccountInfo",
			  "E": 1608545107403,
			  "m": 9,
			  "t": 10,
			  "b": 0,
			  "s": 0,
			  "T": true,
			  "W": true,
			  "D": true,
			  "u": 1608545107401,
			  "B": [
				{
				  "a": "BNB",
				  "f": "89.99345616",
				  "l": "12.00000000"
				},
				{
				  "a": "USDT",
				  "f": "0.16410063",
				  "l": "0.00000000"
				}
			  ],
			  "P": []
			}`,
		},
		{
			input: `{
				"e":"outboundAccountPosition",
				"E":1608545107403,
				"u":1608545107401,
				"B":[{"a":"BNB","f":"89.99345616","l":"12.00000000"},{"a":"USDT","f":"0.16410063","l":"0.00000000"}]
			}`,
		},
	}

	for _, testcase := range testcases {
		payload := testcase.input
		payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")
		event, err := ParseEvent(payload)
		assert.NoError(t, err)
		assert.NotNil(t, event)
	}
}

func TestParseOrderUpdate(t *testing.T) {
	payload := `{
  "e": "executionReport",        // Event type
  "E": 1499405658658,            // Event time
  "s": "ETHBTC",                 // Symbol
  "c": "mUvoqJxFIILMdfAW5iGSOW", // Client order ID
  "S": "BUY",                    // Side
  "o": "LIMIT",                  // Order type
  "f": "GTC",                    // Time in force
  "q": "1.00000000",             // Order quantity
  "p": "0.10264410",             // Order price
  "P": "0.00000000",             // Stop price
  "F": "0.00000000",             // Iceberg quantity
  "g": -1,                       // OrderListId
  "C": null,                     // Original client order ID; This is the ID of the order being canceled
  "x": "NEW",                    // Current execution type
  "X": "NEW",                    // Current order status
  "r": "NONE",                   // Order reject reason; will be an error code.
  "i": 4293153,                  // Order ID
  "l": "0.00000000",             // Last executed quantity
  "z": "0.00000000",             // Cumulative filled quantity
  "L": "0.00000000",             // Last executed price
  "n": "0",                      // Commission amount
  "N": null,                     // Commission asset
  "T": 1499405658657,            // Transaction time
  "t": -1,                       // Trade ID
  "I": 8641984,                  // Ignore
  "w": true,                     // Is the order on the book?
  "m": false,                    // Is this trade the maker side?
  "M": false,                    // Ignore
  "O": 1499405658657,            // Order creation time
  "Z": "0.00000000",             // Cumulative quote asset transacted quantity
  "Y": "0.00000000",              // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
  "Q": "0.00000000"              // Quote Order Qty
}`

	payload = jsCommentTrimmer.ReplaceAllLiteralString(payload, "")

	event, err := ParseEvent(payload)
	assert.NoError(t, err)
	assert.NotNil(t, event)

	executionReport, ok := event.(*ExecutionReportEvent)
	assert.True(t, ok)
	assert.NotNil(t, executionReport)

	orderUpdate, err := executionReport.Order()
	assert.NoError(t, err)
	assert.NotNil(t, orderUpdate)
}
