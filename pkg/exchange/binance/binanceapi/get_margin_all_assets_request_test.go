package binanceapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
)

func TestGetMarginAllAssetsRequest(t *testing.T) {
	for _, asset := range []string{"", "XYZ"} {
		t.Run("asset="+asset, func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			client := NewClient("https://api.binance.test")
			client.Auth("test-key", "test-secret", nil)
			client.HttpClient = &http.Client{Transport: transport}
			transport.GET("/sapi/v1/margin/allAssets", func(req *http.Request) (*http.Response, error) {
				require.Equal(t, asset, req.URL.Query().Get("asset"))
				require.Equal(t, asset != "", req.URL.Query().Has("asset"))
				return httptesting.BuildResponseString(http.StatusOK, `[{
					"assetFullName":"XYZ Coin","assetName":"XYZ",
					"isBorrowable":true,"isMortgageable":false,
					"userMinBorrow":"100.00000000","userMinRepay":"0.10000000",
					"delistTime":1793756994000
				}]`), nil
			})

			req := client.NewGetMarginAllAssetsRequest()
			if asset != "" {
				req.Asset(asset)
			}
			assets, err := req.Do(context.Background())

			require.NoError(t, err)
			minBorrow := fixedpoint.MustNewFromString("100")
			require.Equal(t, []MarginAssetInfo{{
				AssetFullName: "XYZ Coin", AssetName: "XYZ",
				IsBorrowable: true, IsMortgageable: false,
				UserMinBorrow: &minBorrow,
				UserMinRepay:  fixedpoint.MustNewFromString("0.1"),
				DelistTime:    1793756994000,
			}}, assets)
		})
	}
}
