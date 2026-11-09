package evm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

// EndpointPool holds several RPC endpoints for one chain and fails over to the
// next endpoint when a call fails with a retryable (transport or rate limit)
// error. Endpoints are dialed lazily and the last known good endpoint is kept
// sticky so that a healthy primary is not rotated away from.
type EndpointPool struct {
	endpoints []string

	mu      sync.Mutex
	clients []*ethclient.Client
	current int
}

func NewEndpointPool(endpoints []string) (*EndpointPool, error) {
	cleaned := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}

		cleaned = append(cleaned, ep)
	}

	if len(cleaned) == 0 {
		return nil, errors.New("at least one rpc endpoint is required")
	}

	return &EndpointPool{
		endpoints: cleaned,
		clients:   make([]*ethclient.Client, len(cleaned)),
	}, nil
}

func (p *EndpointPool) Len() int {
	return len(p.endpoints)
}

// Current returns the redacted URL of the endpoint currently preferred.
func (p *EndpointPool) Current() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return RedactEndpoint(p.endpoints[p.current])
}

// Endpoints returns the redacted endpoint URLs, safe for logging.
func (p *EndpointPool) Endpoints() []string {
	out := make([]string, 0, len(p.endpoints))
	for _, ep := range p.endpoints {
		out = append(out, RedactEndpoint(ep))
	}

	return out
}

func (p *EndpointPool) client(ctx context.Context, idx int) (*ethclient.Client, error) {
	p.mu.Lock()
	if c := p.clients[idx]; c != nil {
		p.mu.Unlock()
		return c, nil
	}
	endpoint := p.endpoints[idx]
	p.mu.Unlock()

	c, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to dial rpc endpoint %s: %w", RedactEndpoint(endpoint), err)
	}

	p.mu.Lock()
	// another goroutine may have dialed the same endpoint in the meantime
	if existing := p.clients[idx]; existing != nil {
		p.mu.Unlock()
		c.Close()
		return existing, nil
	}
	p.clients[idx] = c
	p.mu.Unlock()

	return c, nil
}

func (p *EndpointPool) markCurrent(idx int) {
	p.mu.Lock()
	p.current = idx
	p.mu.Unlock()
}

// Do runs fn against the preferred endpoint and retries on the remaining
// endpoints when the error is retryable. Non-retryable errors, such as a
// contract revert, are returned immediately: retrying them on another endpoint
// only wastes requests.
func (p *EndpointPool) Do(ctx context.Context, fn func(client *ethclient.Client) error) error {
	p.mu.Lock()
	start := p.current
	p.mu.Unlock()

	n := len(p.endpoints)
	var errs []error

	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		idx := (start + i) % n

		c, err := p.client(ctx, idx)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = fn(c)
		if err == nil {
			if idx != start {
				log.Infof("evm: switched to rpc endpoint %s", RedactEndpoint(p.endpoints[idx]))
			}

			p.markCurrent(idx)
			return nil
		}

		if !IsRetryable(err) {
			p.markCurrent(idx)
			return err
		}

		errs = append(errs, fmt.Errorf("endpoint %s: %w", RedactEndpoint(p.endpoints[idx]), err))
	}

	return fmt.Errorf("all %d rpc endpoints failed: %w", n, errors.Join(errs...))
}

func (p *EndpointPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, c := range p.clients {
		if c != nil {
			c.Close()
			p.clients[i] = nil
		}
	}
}

// nonRetryableMessages are substrings of RPC error messages that describe a
// deterministic rejection: the same request will be rejected by every endpoint.
var nonRetryableMessages = []string{
	"execution reverted",
	"invalid argument",
	"invalid opcode",
	"insufficient funds",
	"nonce too low",
	"already known",
	"replacement transaction underpriced",
	"intrinsic gas too low",
	"gas required exceeds allowance",
}

// IsRetryable reports whether a failed RPC call is worth retrying on another
// endpoint.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// a revert carries return data, which is a deterministic result
	var dataErr rpc.DataError
	if errors.As(err, &dataErr) {
		return false
	}

	msg := strings.ToLower(err.Error())
	for _, s := range nonRetryableMessages {
		if strings.Contains(msg, s) {
			return false
		}
	}

	return true
}

// RedactEndpoint strips credentials from an RPC URL so it can be logged. Many
// providers put the API key in the path or in the query string.
func RedactEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "<invalid-endpoint>"
	}

	var sb strings.Builder
	if u.Scheme != "" {
		sb.WriteString(u.Scheme)
		sb.WriteString("://")
	}

	if u.User != nil {
		sb.WriteString("***@")
	}

	sb.WriteString(u.Host)

	// the last path segment is commonly the API key
	if trimmed := strings.Trim(u.Path, "/"); trimmed != "" {
		segments := strings.Split(trimmed, "/")
		segments[len(segments)-1] = "***"
		sb.WriteString("/")
		sb.WriteString(strings.Join(segments, "/"))
	}

	if u.RawQuery != "" {
		sb.WriteString("?***")
	}

	return sb.String()
}
