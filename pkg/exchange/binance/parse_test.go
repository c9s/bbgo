package binance

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var jsCommentTrimmer = regexp.MustCompile("(?m)//.*$")

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
