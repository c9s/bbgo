package binance

import (
	"context"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
)

func TestQueryMarginAssetInfo(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantErr     string
		zeroMin     bool
		wantWarning bool
	}{
		{name: "matches requested asset", status: http.StatusOK, body: `[{"assetName":"ETH","userMinBorrow":"1"},{"assetName":"DOGE","isBorrowable":true,"userMinBorrow":"100","userMinRepay":"0.1"}]`},
		{name: "missing minimum", status: http.StatusOK, body: `[{"assetName":"DOGE","isBorrowable":true,"userMinRepay":"0.1"}]`, zeroMin: true, wantWarning: true},
		{name: "null minimum", status: http.StatusOK, body: `[{"assetName":"DOGE","isBorrowable":true,"userMinBorrow":null,"userMinRepay":"0.1"}]`, zeroMin: true, wantWarning: true},
		{name: "explicit zero minimum", status: http.StatusOK, body: `[{"assetName":"DOGE","isBorrowable":true,"userMinBorrow":"0","userMinRepay":"0.1"}]`, zeroMin: true},
		{name: "missing asset", status: http.StatusOK, body: `[]`, wantErr: "not found"},
		{name: "negative minimum", status: http.StatusOK, body: `[{"assetName":"DOGE","userMinBorrow":"-1"}]`, wantErr: "invalid minimum"},
		{name: "request failure", status: http.StatusBadRequest, body: `{"code":-1000,"msg":"unavailable"}`, wantErr: "unavailable"},
		{name: "malformed response", status: http.StatusOK, body: `[`, wantErr: "unexpected end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, hook := logrustest.NewNullLogger()
			originalLog := log
			log = logrus.NewEntry(logger)
			t.Cleanup(func() { log = originalLog })
			transport := &httptesting.MockTransport{}
			ex := &Exchange{client2: binanceapi.NewClient("https://api.binance.test")}
			ex.client2.Auth("test-key", "test-secret", nil)
			ex.client2.HttpClient = &http.Client{Transport: transport}
			transport.GET("/sapi/v1/margin/allAssets", func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "DOGE", req.URL.Query().Get("asset"))
				return httptesting.BuildResponseString(tt.status, tt.body), nil
			})

			info, err := ex.QueryMarginAssetInfo(context.Background(), "DOGE")

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				minBorrow := fixedpoint.MustNewFromString("100")
				if tt.zeroMin {
					minBorrow = fixedpoint.Zero
				}
				require.Equal(t, types.MarginAssetInfo{
					Asset: "DOGE", IsBorrowable: true, MinBorrow: minBorrow,
					MinRepay: fixedpoint.MustNewFromString("0.1"),
				}, info)
			}
			if tt.wantWarning {
				require.NotNil(t, hook.LastEntry())
				require.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
				require.Contains(t, hook.LastEntry().Message, "userMinBorrow is missing or null")
				require.Equal(t, "DOGE", hook.LastEntry().Data["asset"])
			} else {
				require.Empty(t, hook.AllEntries())
			}
		})
	}
}
