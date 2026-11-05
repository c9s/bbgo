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

// captureLogEntries hooks the package logger instead of replacing it, because the server time
// goroutine started by New() reads it concurrently and logs unrelated errors.
func captureLogEntries(t *testing.T) *logrustest.Hook {
	hook := &logrustest.Hook{}
	original := log.Logger.ReplaceHooks(make(logrus.LevelHooks))
	hooks := make(logrus.LevelHooks)
	for level, levelHooks := range original {
		hooks[level] = append([]logrus.Hook(nil), levelHooks...)
	}
	hooks.Add(hook)
	log.Logger.ReplaceHooks(hooks)
	t.Cleanup(func() { log.Logger.ReplaceHooks(original) })
	return hook
}

func entriesForAsset(hook *logrustest.Hook, asset string) []logrus.Entry {
	var entries []logrus.Entry
	for _, entry := range hook.AllEntries() {
		if entry.Data["asset"] == asset {
			entries = append(entries, *entry)
		}
	}
	return entries
}

func TestQueryMarginAssetInfo(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		wantErr     string
		zeroMin     bool
		wantWarning bool
	}{
		{name: "matches requested asset", status: http.StatusOK, body: `[{"assetName":"ETH","userMinBorrow":"1"},{"assetName":"XYZ","isBorrowable":true,"userMinBorrow":"100","userMinRepay":"0.1"}]`},
		{name: "missing minimum", status: http.StatusOK, body: `[{"assetName":"XYZ","isBorrowable":true,"userMinRepay":"0.1"}]`, zeroMin: true, wantWarning: true},
		{name: "null minimum", status: http.StatusOK, body: `[{"assetName":"XYZ","isBorrowable":true,"userMinBorrow":null,"userMinRepay":"0.1"}]`, zeroMin: true, wantWarning: true},
		{name: "explicit zero minimum", status: http.StatusOK, body: `[{"assetName":"XYZ","isBorrowable":true,"userMinBorrow":"0","userMinRepay":"0.1"}]`, zeroMin: true},
		{name: "missing asset", status: http.StatusOK, body: `[]`, wantErr: "not found"},
		{name: "negative minimum", status: http.StatusOK, body: `[{"assetName":"XYZ","userMinBorrow":"-1"}]`, wantErr: "invalid minimum"},
		{name: "request failure", status: http.StatusBadRequest, body: `{"code":-1000,"msg":"unavailable"}`, wantErr: "unavailable"},
		{name: "malformed response", status: http.StatusOK, body: `[`, wantErr: "unexpected end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := captureLogEntries(t)
			transport := &httptesting.MockTransport{}
			ex := &Exchange{client2: binanceapi.NewClient("https://api.binance.test")}
			ex.client2.Auth("test-key", "test-secret", nil)
			ex.client2.HttpClient = &http.Client{Transport: transport}
			transport.GET("/sapi/v1/margin/allAssets", func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "XYZ", req.URL.Query().Get("asset"))
				return httptesting.BuildResponseString(tt.status, tt.body), nil
			})

			info, err := ex.QueryMarginAssetInfo(context.Background(), "XYZ")

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				minBorrow := fixedpoint.MustNewFromString("100")
				if tt.zeroMin {
					minBorrow = fixedpoint.Zero
				}
				require.Equal(t, types.MarginAssetInfo{
					Asset: "XYZ", IsBorrowable: true, MinBorrow: minBorrow,
					MinRepay: fixedpoint.MustNewFromString("0.1"),
				}, info)
			}
			entries := entriesForAsset(hook, "XYZ")
			if tt.wantWarning {
				require.Len(t, entries, 1)
				require.Equal(t, logrus.WarnLevel, entries[0].Level)
				require.Contains(t, entries[0].Message, "userMinBorrow is missing or null")
			} else {
				require.Empty(t, entries)
			}
		})
	}
}

func TestQueryMarginAssetInfos(t *testing.T) {
	transport := &httptesting.MockTransport{}
	ex := &Exchange{client2: binanceapi.NewClient("https://api.binance.test")}
	ex.client2.Auth("test-key", "test-secret", nil)
	ex.client2.HttpClient = &http.Client{Transport: transport}
	transport.GET("/sapi/v1/margin/allAssets", func(req *http.Request) (*http.Response, error) {
		require.Empty(t, req.URL.Query().Get("asset"))
		return httptesting.BuildResponseString(http.StatusOK, `[
			{"assetName":"ETH","isBorrowable":true,"userMinBorrow":"0.01","userMinRepay":"0.001"},
			{"assetName":"BTC","isBorrowable":true,"userMinBorrow":"-1"},
			{"assetName":"USDT","isBorrowable":true,"userMinBorrow":"5"}
		]`), nil
	})

	infos, err := ex.QueryMarginAssetInfos(context.Background(), []string{"ETH", "BTC", "LTC"})

	require.NoError(t, err)
	require.Equal(t, map[string]types.MarginAssetInfo{
		"ETH": {
			Asset: "ETH", IsBorrowable: true,
			MinBorrow: fixedpoint.MustNewFromString("0.01"),
			MinRepay:  fixedpoint.MustNewFromString("0.001"),
		},
	}, infos, "unrequested USDT, invalid BTC and unlisted LTC are left out")
}
