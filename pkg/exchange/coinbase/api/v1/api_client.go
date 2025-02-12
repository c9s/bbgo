package coinbase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

const (
	DefaultHTTPTimeout = 15 * time.Second
	ProductionAPIURL   = "https://api.exchange.coinbase.com"
)

var parsedBaseURL *url.URL

func init() {
	url, err := url.Parse(ProductionAPIURL)
	if err != nil {
		panic(err)
	}
	parsedBaseURL = url
}

type RestAPIClient struct {
	requestgen.BaseAPIClient

	key, secret, passphrase string
}

func NewClient(
	key, secret, passphrase string,
) RestAPIClient {
	return RestAPIClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL:    parsedBaseURL,
			HttpClient: &http.Client{Timeout: DefaultHTTPTimeout},
		},
		key:        key,
		secret:     secret,
		passphrase: passphrase,
	}
}

// Implements AuthenticatedAPIClient
func (client *RestAPIClient) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	return client.newAuthenticatedRequest(ctx, method, refURL, time.Now(), params, payload)
}

func (client *RestAPIClient) newAuthenticatedRequest(ctx context.Context, method, refURL string, timestamp time.Time, params url.Values, payload interface{}) (*http.Request, error) {
	req, err := client.NewRequest(ctx, method, refURL, params, payload)
	if err != nil {
		return nil, err
	}

	tsStr := strconv.FormatInt(timestamp.Unix(), 10)
	signature, err := sign(client.secret, tsStr, method, req.URL.RequestURI(), payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("CB-ACCESS-KEY", client.key)
	req.Header.Add("CB-ACCESS-SIGN", signature)
	req.Header.Add("CB-ACCESS-TIMESTAMP", tsStr)
	req.Header.Add("CB-ACCESS-PASSPHRASE", client.passphrase)

	return req, nil
}

// https://docs.cdp.coinbase.com/exchange/docs/rest-auth#signing-a-message
func sign(secret, timestamp, method, requestPath string, body interface{}) (string, error) {
	var bodyStr string
	if body != nil {
		bodyByte, err := json.Marshal(body)
		if err != nil {
			return "", errors.Wrap(err, "failed to marshal body to string")
		}

		bodyStr = string(bodyByte)
	}

	message := timestamp + method + requestPath + bodyStr

	secretByte, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode secret string to byte")
	}
	mac := hmac.New(sha256.New, secretByte)
	_, err = mac.Write([]byte(message))
	if err != nil {
		return "", errors.Wrap(err, "failed to encode message")
	}
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature, nil
}
