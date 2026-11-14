package btc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRPCClientURLCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":975765,"id":1}`)
	}))
	defer srv.Close()

	c, err := NewClient("http://alice:secret@" + srv.Listener.Addr().String())
	require.NoError(t, err)
	assert.NotContains(t, c.Endpoint(), "secret")

	height, err := c.GetBlockCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(975765), height)

	bad, err := NewClient("http://alice:wrong@" + srv.Listener.Addr().String())
	require.NoError(t, err)

	_, err = bad.GetBlockCount(context.Background())
	assert.ErrorContains(t, err, "authentication failed")
}

func TestRPCClientCookieRotation(t *testing.T) {
	cookieFile := filepath.Join(t.TempDir(), ".cookie")
	require.NoError(t, os.WriteFile(cookieFile, []byte("__cookie__:old\n"), 0o600))

	var current atomic.Value
	current.Store("old")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "__cookie__" || pass != current.Load().(string) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":1,"id":1}`)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, WithCookieFile(cookieFile))
	require.NoError(t, err)

	_, err = c.GetBlockCount(context.Background())
	require.NoError(t, err)

	// the node restarts and writes a new cookie
	current.Store("new")
	require.NoError(t, os.WriteFile(cookieFile, []byte("__cookie__:new"), 0o600))

	_, err = c.GetBlockCount(context.Background())
	require.NoError(t, err)
}

func TestRPCClientErrorDialects(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		code    int
		message string
	}{
		{
			name:   "json-rpc 2.0 (Core 28+)",
			status: http.StatusOK,
			body:   `{"jsonrpc":"2.0","error":{"code":-5,"message":"Block not found"},"id":1}`,
			code:   -5,
		},
		{
			name:   "legacy with http 500",
			status: http.StatusInternalServerError,
			body:   `{"result":null,"error":{"code":-8,"message":"Block height out of range"},"id":1}`,
			code:   -8,
		},
		{
			name:   "legacy method not found with http 404",
			status: http.StatusNotFound,
			body:   `{"result":null,"error":{"code":-32601,"message":"Method not found"},"id":1}`,
			code:   -32601,
		},
		{
			name:    "not json",
			status:  http.StatusServiceUnavailable,
			body:    "<html>Service Unavailable</html>",
			message: "http 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			c, err := NewClient(srv.URL)
			require.NoError(t, err)

			_, err = c.GetBlockHash(context.Background(), 1)
			require.Error(t, err)

			if tt.code != 0 {
				var rpcErr *RPCError
				require.ErrorAs(t, err, &rpcErr)
				assert.Equal(t, tt.code, rpcErr.Code)
				assert.Equal(t, tt.code == ErrCodeInvalidAddressOrKey, IsNotFound(err))
			} else {
				assert.ErrorContains(t, err, tt.message)
			}
		})
	}
}

func TestRPCClientRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","result":"`+genesisHash+`","id":1}`)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	require.NoError(t, err)

	h, err := c.GetBlockHash(context.Background(), 0)
	require.NoError(t, err)
	assert.Equal(t, genesisHash, h.String())

	assert.Equal(t, "2.0", got["jsonrpc"])
	assert.Equal(t, "getblockhash", got["method"])
	assert.Equal(t, []any{float64(0)}, got["params"])

	// a method without params sends an empty array, not null
	_, err = c.GetBestBlockHash(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []any{}, got["params"])
}

func TestRPCClientBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []rpcRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqs))
		require.Len(t, reqs, 3)

		// answered out of order, one with an error, the third not at all
		resps := []map[string]any{
			{"jsonrpc": "2.0", "id": reqs[1].ID, "error": map[string]any{"code": -8, "message": "Block height out of range"}},
			{"jsonrpc": "2.0", "id": reqs[0].ID, "result": 975765},
		}
		require.NoError(t, json.NewEncoder(w).Encode(resps))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	require.NoError(t, err)

	var height int64
	var hash, other Hash
	batch := []BatchElem{
		{Method: "getblockcount", Result: &height},
		{Method: "getblockhash", Params: []any{99999999}, Result: &hash},
		{Method: "getbestblockhash", Result: &other},
	}

	require.NoError(t, c.BatchCall(context.Background(), batch))

	assert.NoError(t, batch[0].Error)
	assert.Equal(t, int64(975765), height)

	var rpcErr *RPCError
	require.ErrorAs(t, batch[1].Error, &rpcErr)
	assert.Equal(t, ErrCodeInvalidParameter, rpcErr.Code)

	assert.ErrorContains(t, batch[2].Error, "no response")
}

func TestRPCClientBatchRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","error":{"code":-32600,"message":"batch not allowed"},"id":null}`)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	require.NoError(t, err)

	err = c.BatchCall(context.Background(), []BatchElem{{Method: "getblockcount"}})
	assert.ErrorContains(t, err, "batch not allowed")
}

func TestNewRPCClientInvalidEndpoint(t *testing.T) {
	_, err := NewClient("ws://127.0.0.1:8332")
	assert.Error(t, err)

	_, err = NewClient("://bad")
	assert.Error(t, err)
}
