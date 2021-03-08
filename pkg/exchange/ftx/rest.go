package ftx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/util"
)

type restRequest struct {
	*balanceRequest
	*orderRequest

	key, secret string
	// Optional sub-account name
	sub string

	c       *http.Client
	baseURL *url.URL
	refURL  string
	// http method, e.g., GET or POST
	m string

	// payload
	p map[string]interface{}
}

func newRestRequest(c *http.Client, baseURL *url.URL) *restRequest {
	r := &restRequest{
		c:       c,
		baseURL: baseURL,
	}

	r.balanceRequest = &balanceRequest{restRequest: r}
	r.orderRequest = &orderRequest{restRequest: r}
	return r
}

func (r *restRequest) Auth(key, secret string) *restRequest {
	r.key = key
	r.secret = secret
	return r
}

func (r *restRequest) SubAccount(subAccount string) *restRequest {
	r.sub = subAccount
	return r
}

func (r *restRequest) Method(method string) *restRequest {
	r.m = method
	return r
}

func (r *restRequest) ReferenceURL(refURL string) *restRequest {
	r.refURL = refURL
	return r
}

func (r *restRequest) buildURL() (*url.URL, error) {
	refURL, err := url.Parse(r.refURL)
	if err != nil {
		return nil, err
	}
	return r.baseURL.ResolveReference(refURL), nil
}

func (r *restRequest) Payloads(payloads map[string]interface{}) *restRequest {
	r.p = make(map[string]interface{})
	for k, v := range payloads {
		r.p[k] = v
	}
	return r
}

func (r *restRequest) DoAuthenticatedRequest(ctx context.Context) (*util.Response, error) {
	req, err := r.newAuthenticatedRequest(ctx)
	if err != nil {
		return nil, err
	}

	return r.sendRequest(req)
}

func (r *restRequest) newAuthenticatedRequest(ctx context.Context) (*http.Request, error) {
	u, err := r.buildURL()
	if err != nil {
		return nil, err
	}

	var jsonPayload []byte
	if len(r.p) > 0 {
		var err2 error
		jsonPayload, err2 = json.Marshal(r.p)
		if err2 != nil {
			return nil, fmt.Errorf("can't marshal payload map to json: %w", err2)
		}
	}

	ts := strconv.FormatInt(timestamp(), 10)
	p := fmt.Sprintf("%s%s%s%s", ts, r.m, u.Path, jsonPayload)
	signature := sign(r.secret, p)

	req, err := http.NewRequestWithContext(ctx, r.m, u.String(), bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("FTX-KEY", r.key)
	req.Header.Set("FTX-SIGN", signature)
	req.Header.Set("FTX-TS", ts)
	if r.sub != "" {
		req.Header.Set("FTX-SUBACCOUNT", r.sub)
	}

	return req, nil
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func timestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (r *restRequest) sendRequest(req *http.Request) (*util.Response, error) {
	resp, err := r.c.Do(req)
	if err != nil {
		return nil, err
	}

	// newResponse reads the response body and return a new Response object
	response, err := util.NewResponse(resp)
	if err != nil {
		return response, err
	}

	// Check error, if there is an error, return the ErrorResponse struct type
	if response.IsError() {
		errorResponse, err := toErrorResponse(response)
		if err != nil {
			return response, err
		}
		return response, errorResponse
	}

	return response, nil
}

type ErrorResponse struct {
	*util.Response

	IsSuccess   bool   `json:"Success"`
	ErrorString string `json:"error,omitempty"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%s %s %d, Success: %t, err: %s",
		r.Response.Request.Method,
		r.Response.Request.URL.String(),
		r.Response.StatusCode,
		r.IsSuccess,
		r.ErrorString,
	)
}

func toErrorResponse(response *util.Response) (*ErrorResponse, error) {
	errorResponse := &ErrorResponse{Response: response}

	if response.IsJSON() {
		var err = response.DecodeJSON(errorResponse)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode json for response: %d %s", response.StatusCode, string(response.Body))
		}

		if errorResponse.IsSuccess {
			return nil, fmt.Errorf("response.Success should be false")
		}
		return errorResponse, nil
	}

	return errorResponse, fmt.Errorf("unexpected response content type %s", response.Header.Get("content-type"))
}
