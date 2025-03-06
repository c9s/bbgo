package coinbase

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SubCmdString(t *testing.T) {
	var subCmd interface{}
	subCmd = subscribeMsgType1{
		Type: "subscribe",
		Channels: []channelType{
			{
				Name:       "ticker",
				ProductIDs: []string{"BTC-USD"},
			},
		},
		authMsg: authMsg{
			Signature:  "signature",
			Key:        "<secret_key!>",
			Passphrase: "<secret_passphrase!>",
			Timestamp:  "timestamp",
		},
	}
	outStr := fmt.Sprintf("%s", subCmd)
	assert.False(t, strings.Contains(outStr, "<secret_key!>"))
	assert.False(t, strings.Contains(outStr, "<secret_passphrase!>"))

	subCmd = subscribeMsgType2{
		Type:       "subscribe",
		Channels:   []string{},
		ProductIDs: []string{},
		AccountIDs: []string{},
		authMsg: authMsg{
			Signature:  "signature",
			Key:        "<secret_key!>",
			Passphrase: "<secret_passphrase!>",
			Timestamp:  "timestamp",
		},
	}
	outStr = fmt.Sprintf("%s", subCmd)
	assert.False(t, strings.Contains(outStr, "<secret_key!>"))
	assert.False(t, strings.Contains(outStr, "<secret_passphrase!>"))
}
