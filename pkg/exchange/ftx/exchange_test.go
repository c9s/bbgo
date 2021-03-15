package ftx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestExchange_QueryAccountBalances(t *testing.T) {
	successResp := `
{
  "result": [
    {
      "availableWithoutBorrow": 19.47458865,
      "coin": "USD",
      "free": 19.48085209,
      "spotBorrow": 0.0,
      "total": 1094.66405065,
      "usdValue": 1094.664050651561
    }
  ],
  "success": true
}
`
	failureResp := `{"result":[],"success":false}`
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i == 0 {
			fmt.Fprintln(w, successResp)
			i++
			return
		}
		fmt.Fprintln(w, failureResp)
	}))
	defer ts.Close()

	ex := NewExchange("", "", "")
	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.restEndpoint = serverURL
	resp, err := ex.QueryAccountBalances(context.Background())
	assert.NoError(t, err)

	assert.Len(t, resp, 1)
	b, ok := resp["USD"]
	assert.True(t, ok)
	expectedAvailable := fixedpoint.Must(fixedpoint.NewFromString("19.48085209"))
	assert.Equal(t, expectedAvailable, b.Available)
	assert.Equal(t, fixedpoint.Must(fixedpoint.NewFromString("1094.66405065")).Sub(expectedAvailable), b.Locked)

	resp, err = ex.QueryAccountBalances(context.Background())
	assert.EqualError(t, err, "ftx returns querying balances failure")
	assert.Nil(t, resp)
}
