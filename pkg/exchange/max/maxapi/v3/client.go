package v3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/version"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "max",
	"api":      "v3",
})

const (
	// ProductionAPIURL is the official MAX API v2 Endpoint
	ProductionAPIURL = "https://max-api.maicoin.com/api/v3"

	UserAgent = "bbgo/" + version.Version

	defaultHTTPTimeout = time.Second * 60

	// 2018-09-01 08:00:00 +0800 CST
	TimestampSince = 1535760000

	maxAllowedNegativeTimeOffset = -20
)

// The following variables are used for nonce.

// ServerTimeOffset is used for nonce
var ServerTimeOffset int64 = 0

// ServerTimestamp is used for storing the server timestamp, default to Now
var ServerTimestamp = time.Now().Unix()

// reqCount is used for nonce, this variable counts the API request count.
var reqCount uint64 = 1

var disableUserAgentHeader = false

// create type alias
type WalletType = maxapi.WalletType
type OrderByType = maxapi.OrderByType
type OrderType = maxapi.OrderType

type Order = maxapi.Order
type Account = maxapi.Account

func init() {
	UpdateServerTimeAndOffset(context.Background())
}

// UpdateServerTimeAndOffset updates the global server time and offset variables
// by querying the MAX server timestamp API endpoint.
// This function will keep retrying until it succeeds or the context is done.
func UpdateServerTimeAndOffset(baseCtx context.Context) {
	client := maxapi.NewRestClientDefault()
	v3client := NewClient(client)
	ctx, cancel := context.WithTimeout(baseCtx, time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("unable to update server time offset due to timeout")
			return
		default:
		}

		resp, err := v3client.NewGetTimestampRequest().Do(ctx)
		if err != nil {
			log.WithError(err).Errorf("unable to get max exchange server timestamp")
			continue
		}

		clientTime := time.Now()
		serverTime := time.Unix(resp.TimestampInSeconds, 0)
		offset := serverTime.Unix() - clientTime.Unix()

		if offset < 0 {
			// avoid updating a negative offset: server time is before our local time
			if offset > maxAllowedNegativeTimeOffset {
				return
			}

			// if the offset is greater than 15 seconds, we should restart
			log.Panicf("max exchange server timestamp offset %d is less than the negative offset %d", offset, maxAllowedNegativeTimeOffset)
		}

		atomic.StoreInt64(&ServerTimestamp, serverTime.Unix())
		atomic.StoreInt64(&ServerTimeOffset, offset)
		log.Infof("max exchange server timestamp offset %d", offset)
		return
	}
}

type Client struct {
	*maxapi.RestClient

	MarginService     *MarginService
	SubAccountService *SubAccountService
}

func NewClient(legacyClient *maxapi.RestClient) *Client {
	client := &Client{
		RestClient:        legacyClient,
		MarginService:     &MarginService{Client: legacyClient},
		SubAccountService: &SubAccountService{Client: legacyClient},
	}

	return client
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
func (c *Client) NewAuthenticatedRequest(
	ctx context.Context, m string, refURL string, params url.Values, data interface{},
) (*http.Request, error) {
	if len(c.RestClient.APIKey) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(c.RestClient.APISecret) == 0 {
		return nil, errors.New("empty api secret")
	}

	apiKey := c.RestClient.APIKey
	apiSecret := c.RestClient.APISecret
	if c.RestClient.ApiKeyRotator != nil {
		apiKey, apiSecret = c.RestClient.ApiKeyRotator.Next().GetKeySecret()
	}

	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	n := c.RestClient.GetNonce(apiKey)
	payload := map[string]interface{}{
		"nonce": n,
		"path":  c.RestClient.BaseURL.ResolveReference(rel).Path,
	}

	switch d := data.(type) {
	case map[string]interface{}:
		for k, v := range d {
			payload[k] = v
		}
	default:
		log.Warnf("unsupported payload type: %T", d)
	}

	for k, vs := range params {
		k = strings.TrimSuffix(k, "[]")
		if len(vs) == 1 {
			payload[k] = vs[0]
		} else {
			payload[k] = vs
		}
	}

	p, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := c.RestClient.NewRequest(ctx, m, refURL, params, p)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(p)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MAX-ACCESSKEY", apiKey)
	req.Header.Add("X-MAX-PAYLOAD", encoded)
	req.Header.Add("X-MAX-SIGNATURE", signPayload(encoded, apiSecret))
	if c.RestClient.SubAccount != "" {
		req.Header.Add("X-Sub-Account", c.RestClient.SubAccount)
	}

	if disableUserAgentHeader {
		req.Header.Set("USER-AGENT", "")
		req.Header.Del("User-Agent")
	} else {
		req.Header.Set("USER-AGENT", UserAgent)
	}

	if false {
		out, _ := httputil.DumpRequestOut(req, true)
		fmt.Println(string(out))
	}

	return req, nil
}

func castPayload(payload interface{}) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	switch v := payload.(type) {
	case string:
		return []byte(v), nil

	case []byte:
		return v, nil
	}

	body, err := json.Marshal(payload)
	return body, err
}

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}
