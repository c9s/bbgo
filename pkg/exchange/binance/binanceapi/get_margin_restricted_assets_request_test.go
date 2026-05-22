package binanceapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMarginRestrictedAssetsRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetMarginRestrictedAssetsRequest()
	resp, err := req.Do(ctx)
	if assert.NoError(t, err) {
		assert.NotNil(t, resp)
		t.Logf("Restricted Assets: %+v", resp)
	}
}
