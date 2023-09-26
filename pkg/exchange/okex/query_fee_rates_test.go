package okex

import (
	"context"
	"testing"

	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func Test_GetAllFeeRates(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.Skip("Please configure all credentials about OKEX")
	}

	e := New(key, secret, passphrase)

	feeRate, err := e.GetAllFeeRates(context.Background())
	if assert.NoError(t, err) {
		assert.NotEmpty(t, feeRate)
	}
	t.Logf("fee rate detail: %+v", feeRate)
}
