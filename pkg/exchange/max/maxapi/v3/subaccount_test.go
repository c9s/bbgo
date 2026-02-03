package v3

import (
	"context"
	"os"
	"testing"
	"time"

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

	client := NewClient()

	isRecording, saveRecord := httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")
	defer saveRecord()

	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if ok {
		client.Auth(key, secret)
	} else {
		client.Auth("foo", "bar")
	}

	subAccount := os.Getenv("MAX_API_SUB_ACCOUNT")
	if subAccount != "" {
		client.SetSubAccount(subAccount)
	}

	if isRecording && !ok {
		t.Skipf("MAX api key is not configured, skipping integration test")
	}

	t.Run("GetSubAccountsRequest", func(t *testing.T) {
		req := client.SubAccountService.NewGetSubAccountsRequest()
		subAccounts, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("subAccounts: %+v", subAccounts)

		// clean up sub-accounts
		if len(subAccounts) > 2 {
			for _, sa := range subAccounts[1:] {
				client.SetSubAccount(sa.SN)

				bals, err := client.NewGetWalletAccountsRequest(maxapi.WalletTypeSpot).Do(ctx)
				if assert.NoError(t, err) {
					for _, b := range bals {
						if b.Balance.IsZero() {
							continue
						}

						t.Logf("transferring %s %s from sub-account %s back to main account", b.Balance.String(), b.Currency, sa.SN)

						transferBackReq := client.SubAccountService.NewSubmitSubAccountTransferRequest()
						transferBackReq.Amount(b.Balance.String()).
							Currency(b.Currency).
							ToMain(true)

						resp2, err := transferBackReq.Do(ctx)
						if assert.NoError(t, err) {
							t.Logf("transfer %s %s response: %+v", b.Balance.String(), b.Currency, resp2)
						}
					}
				}

				client.SetSubAccount("")
				t.Logf("deleting sub-account %s", sa.SN)
				delReq := client.SubAccountService.NewDeleteSubAccountRequest()
				delReq.Sn(sa.SN)
				resp, err := delReq.Do(ctx)
				if assert.NoError(t, err) {
					t.Logf("delete sub-account response: %+v", resp)
				}
			}
		}
	})

	t.Run("CreateSubAccountRequest", func(t *testing.T) {
		client.SetSubAccount("")

		req := client.SubAccountService.NewCreateSubAccountRequest()
		req.Name(uuid.New().String())

		createdSubAccount, err := req.Do(ctx)
		assert.NoError(t, err)
		t.Logf("createdSubAccount: %+v", createdSubAccount)

		transferReq := client.SubAccountService.NewSubmitSubAccountTransferRequest()
		transferReq.Amount("10").
			Currency("usdt").
			ToSN(createdSubAccount.SN)
		resp, err := transferReq.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("transfer response: %+v", resp)
		}

		if assert.NotNil(t, createdSubAccount) {
			client.SetSubAccount(createdSubAccount.SN)

			if isRecording {
				time.Sleep(1 * time.Second)
			}

			transferBackReq := client.SubAccountService.NewSubmitSubAccountTransferRequest()
			transferBackReq.Amount("10").
				Currency("usdt").
				ToMain(true)
			resp2, err := transferBackReq.Do(ctx)
			if assert.NoError(t, err) {
				t.Logf("transfer back response: %+v", resp2)
			}

			if isRecording {
				time.Sleep(1 * time.Second)
			}

			client.SetSubAccount("")
			delReq := client.SubAccountService.NewDeleteSubAccountRequest()
			delReq.Sn(createdSubAccount.SN)
			resp, err := delReq.Do(ctx)
			assert.NoError(t, err)
			t.Logf("delete sub-account response: %+v", resp)
		}
	})
}
