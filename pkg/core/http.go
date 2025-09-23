package core

import (
	"net"
	"net/http"
	"time"

	"github.com/c9s/bbgo/pkg/envvar"
)

const DefaultHTTPTimeout = time.Second * 60

var HttpClient = &http.Client{
	Timeout:   DefaultHTTPTimeout,
	Transport: HttpTransport,
}

var httpTransportMaxIdleConnsPerHost = http.DefaultMaxIdleConnsPerHost

// httpTransportMaxIdleConns is the maximum number of idle (keep-alive) connections across all hosts.
// If zero, DefaultMaxIdleConns is used.
// DefaultMaxIdleConns is 100 (same as http.DefaultTransport)
var httpTransportMaxIdleConns = 100

// httpTransportIdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself.
// The default Idle Timeout values vary based on the type of Elastic Load Balancer:
// Classic Load Balancer (CLB): The default Idle Timeout is 60 seconds.
// Application Load Balancer (ALB): The default Idle Timeout is 60 seconds.
// Network Load Balancer (NLB): The default Idle Timeout is 350 seconds
var httpTransportIdleConnTimeout = 60 * time.Second

// create an isolated http httpTransport rather than the default one
var HttpTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,

	// ForceAttemptHTTP2:     true,
	// DisableCompression:    false,

	MaxIdleConns:          httpTransportMaxIdleConns,
	MaxIdleConnsPerHost:   httpTransportMaxIdleConnsPerHost,
	IdleConnTimeout:       httpTransportIdleConnTimeout,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func init() {
	if val, ok := envvar.Int("HTTP_TRANSPORT_MAX_IDLE_CONNS_PER_HOST"); ok {
		httpTransportMaxIdleConnsPerHost = val
	}

	if val, ok := envvar.Int("HTTP_TRANSPORT_MAX_IDLE_CONNS"); ok {
		httpTransportMaxIdleConns = val
	}

	if val, ok := envvar.Duration("HTTP_TRANSPORT_IDLE_CONN_TIMEOUT"); ok {
		httpTransportIdleConnTimeout = val
	}
}
