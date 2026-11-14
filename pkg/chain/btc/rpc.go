package btc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Bitcoin Core RPC error codes the callers of this package branch on.
// The full list is in Bitcoin Core's src/rpc/protocol.h.
const (
	// ErrCodeInvalidAddressOrKey is also what Core answers for a block or
	// transaction it does not have.
	ErrCodeInvalidAddressOrKey = -5
	ErrCodeInvalidParameter    = -8
	ErrCodeInWarmup            = -28
	ErrCodeMethodNotFound      = -32601
)

// maxResponseSize bounds a response body. A verbosity=3 block of a full 4 MB
// weight is around 15 MB of JSON.
const maxResponseSize = 256 << 20

// RPCError is an error returned by the node.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("btc rpc error %d: %s", e.Code, e.Message)
}

// IsNotFound reports whether err is the node saying the block or transaction
// does not exist.
func IsNotFound(err error) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == ErrCodeInvalidAddressOrKey
}

// RPCClient is a JSON-RPC client for Bitcoin Core and the providers that
// speak its API.
//
// Core answers in two dialects: a JSON-RPC 2.0 request gets a 2.0 response
// from Core 28 on, while older nodes ignore the version and answer the legacy
// way, with both "result" and "error" set and an HTTP 500 for an error. The
// client reads the body whatever the status and handles both.
type RPCClient struct {
	endpoint   string
	httpClient *http.Client

	user, password string
	cookieFile     string

	cookieMu sync.Mutex
	cookie   string

	id atomic.Uint64
}

// RPCOption configures an RPCClient.
type RPCOption func(*RPCClient)

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(c *http.Client) RPCOption {
	return func(rc *RPCClient) {
		rc.httpClient = c
	}
}

// WithBasicAuth sets the rpcuser / rpcpassword (or rpcauth) credentials.
func WithBasicAuth(user, password string) RPCOption {
	return func(rc *RPCClient) {
		rc.user, rc.password = user, password
	}
}

// WithCookieFile authenticates with the .cookie file a local bitcoind writes
// to its data directory. The cookie changes on every restart of the node, so
// it is read again after an authentication failure.
func WithCookieFile(path string) RPCOption {
	return func(rc *RPCClient) {
		rc.cookieFile = path
	}
}

// NewRPCClient creates a client for endpoint. Credentials in the URL
// (http://user:pass@127.0.0.1:8332) are used as basic auth.
func NewRPCClient(endpoint string, options ...RPCOption) (*RPCClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("btc: invalid rpc endpoint: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("btc: rpc endpoint must be http or https, got %q", u.Scheme)
	}

	c := &RPCClient{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	if u.User != nil {
		c.user = u.User.Username()
		c.password, _ = u.User.Password()
		u.User = nil
	}

	c.endpoint = u.String()

	for _, o := range options {
		o(c)
	}

	return c, nil
}

// Endpoint returns the endpoint without credentials.
func (c *RPCClient) Endpoint() string {
	return c.endpoint
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// Call invokes method and decodes its result into result, which may be nil
// to discard it. A JSON null result leaves result untouched; methods with a
// null answer, such as gettxout for a spent output, check for that.
func (c *RPCClient) Call(ctx context.Context, result any, method string, params ...any) error {
	req := c.newRequest(method, params)

	body, err := c.post(ctx, req)
	if err != nil {
		return fmt.Errorf("btc rpc %s: %w", method, err)
	}

	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("btc rpc %s: decode response: %w", method, err)
	}

	return decodeResult(method, &resp, result)
}

// BatchElem is one call of a batch request.
type BatchElem struct {
	Method string
	Params []any

	// Result receives the decoded result, as in Call.
	Result any

	// Error is set when this call failed; the other calls of the batch are
	// not affected.
	Error error
}

// BatchCall sends all calls in one HTTP request. The returned error is about
// the request as a whole; each call's own error is set on its element.
func (c *RPCClient) BatchCall(ctx context.Context, batch []BatchElem) error {
	if len(batch) == 0 {
		return nil
	}

	reqs := make([]rpcRequest, len(batch))
	byID := make(map[uint64]int, len(batch))
	for i := range batch {
		reqs[i] = c.newRequest(batch[i].Method, batch[i].Params)
		byID[reqs[i].ID] = i
	}

	body, err := c.post(ctx, reqs)
	if err != nil {
		return fmt.Errorf("btc rpc batch: %w", err)
	}

	var resps []rpcResponse
	if err := json.Unmarshal(body, &resps); err != nil {
		// a node that rejects the whole batch answers with a single object
		var single rpcResponse
		if json.Unmarshal(body, &single) == nil && single.Error != nil {
			return fmt.Errorf("btc rpc batch: %w", single.Error)
		}

		return fmt.Errorf("btc rpc batch: decode response: %w", err)
	}

	answered := make([]bool, len(batch))
	for i := range resps {
		idx, ok := byID[resps[i].ID]
		if !ok {
			continue
		}

		answered[idx] = true
		batch[idx].Error = decodeResult(batch[idx].Method, &resps[i], batch[idx].Result)
	}

	for i := range batch {
		if !answered[i] {
			batch[i].Error = fmt.Errorf("btc rpc %s: no response in batch", batch[i].Method)
		}
	}

	return nil
}

func (c *RPCClient) newRequest(method string, params []any) rpcRequest {
	if params == nil {
		params = []any{}
	}

	return rpcRequest{JSONRPC: "2.0", ID: c.id.Add(1), Method: method, Params: params}
}

func decodeResult(method string, resp *rpcResponse, result any) error {
	if resp.Error != nil {
		return resp.Error
	}

	if result == nil || len(resp.Result) == 0 || string(resp.Result) == "null" {
		return nil
	}

	if err := json.Unmarshal(resp.Result, result); err != nil {
		return fmt.Errorf("btc rpc %s: decode result: %w", method, err)
	}

	return nil
}

func (c *RPCClient) post(ctx context.Context, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, status, err := c.send(ctx, data)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized && c.cookieFile != "" {
		// the node restarted and rotated its cookie
		c.cookieMu.Lock()
		c.cookie = ""
		c.cookieMu.Unlock()

		if body, status, err = c.send(ctx, data); err != nil {
			return nil, err
		}
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("http %d: authentication failed", status)
	}

	// Legacy Core answers errors with 500 / 404 and a JSON body; only a body
	// that is not JSON makes the status itself the error.
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, fmt.Errorf("http %d: %s", status, truncate(string(trimmed), 200))
	}

	return body, nil
}

func (c *RPCClient) send(ctx context.Context, data []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	user, password, err := c.credentials()
	if err != nil {
		return nil, 0, err
	}

	if user != "" || password != "" {
		req.SetBasicAuth(user, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if len(body) > maxResponseSize {
		return nil, resp.StatusCode, fmt.Errorf("response larger than %d bytes", maxResponseSize)
	}

	return body, resp.StatusCode, nil
}

func (c *RPCClient) credentials() (string, string, error) {
	if c.cookieFile == "" {
		return c.user, c.password, nil
	}

	c.cookieMu.Lock()
	defer c.cookieMu.Unlock()

	if c.cookie == "" {
		data, err := os.ReadFile(c.cookieFile)
		if err != nil {
			return "", "", fmt.Errorf("read rpc cookie: %w", err)
		}

		c.cookie = strings.TrimSpace(string(data))
	}

	user, password, ok := strings.Cut(c.cookie, ":")
	if !ok {
		return "", "", fmt.Errorf("rpc cookie file %s is not user:password", c.cookieFile)
	}

	return user, password, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
