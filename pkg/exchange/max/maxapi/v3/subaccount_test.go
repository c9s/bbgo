package v3

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestSubAccount(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseClient := maxapi.NewRestClientDefault()
	client := NewClient(baseClient)

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, baseClient.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if ok {
		baseClient.Auth(key, secret)
	}

	subAccount := os.Getenv("MAX_API_SUB_ACCOUNT")
	if subAccount != "" {
		baseClient.SetSubAccount(subAccount)
	}

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	t.Run("GetSubAccountsRequest", func(t *testing.T) {
		req := client.SubAccountService.NewGetSubAccountsRequest()
		subAccounts, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("subAccounts: %+v", subAccounts)
	})

	t.Run("CreateSubAccountRequest", func(t *testing.T) {
		req := client.SubAccountService.NewCreateSubAccountRequest()
		req.Name(uuid.New().String())

		createdSubAccount, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("createdSubAccount: %+v", createdSubAccount)

		transferReq := client.SubAccountService.NewSubmitSubAccountTransferRequest()
		transferReq.Amount("10").CompositeCurrency("usdt").ToSN(createdSubAccount.SN)
		resp, err := transferReq.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("transfer response: %+v", resp)
		}

		t.Cleanup(func() {
			if assert.NotNil(t, createdSubAccount) {
				transferBackReq := client.SubAccountService.NewSubmitSubAccountTransferRequest()
				transferBackReq.Amount("10").CompositeCurrency("usdt").ToSN(createdSubAccount.SN).ToMain(true)
				resp2, err := transferReq.Do(ctx)
				if assert.NoError(t, err) {
					t.Logf("transfer response: %+v", resp2)
				}

				delReq := client.SubAccountService.NewDeleteSubAccountRequest()
				delReq.Sn(createdSubAccount.SN)
				resp, err := delReq.Do(ctx)
				assert.NoError(t, err)
				t.Logf("delete sub-account response: %+v", resp)
			}
		})
	})
}
