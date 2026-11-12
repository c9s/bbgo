package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

const (
	dialTimeout = 10 * time.Second

	// a connection that stayed up this long resets the reconnect backoff
	healthyConnection = time.Minute
)

var errStalled = errors.New("no new head within the stall timeout")

// errWrongChain is a configuration error: retrying will not fix it.
type errWrongChain struct {
	expected evm.ChainID
	actual   uint64
}

func (e *errWrongChain) Error() string {
	return fmt.Sprintf("evm stream: the websocket endpoint serves chain id %d, expected %d", e.actual, uint64(e.expected))
}

// newHead decodes only the block number of a newHeads notification. The hash
// is deliberately not used: blocks are read over HTTP and their hashes come
// from there. Decoding a full header would also break on a chain whose header
// lacks a field go-ethereum requires.
type newHead struct {
	Number *hexutil.Big `json:"number"`
}

// subscribeHeads keeps a newHeads subscription alive until ctx is done,
// reconnecting with a backoff. A configuration error is sent to fatal.
func (s *Stream) subscribeHeads(ctx context.Context, fatal chan<- error) {
	backoff := time.Duration(0)

	for {
		started := time.Now()
		err := s.subscribeOnce(ctx)

		if ctx.Err() != nil {
			return
		}

		var wrongChain *errWrongChain
		if errors.As(err, &wrongChain) {
			fatal <- err
			return
		}

		if time.Since(started) > healthyConnection {
			backoff = 0
		}

		backoff = nextBackoff(backoff)
		log.WithError(err).Warnf("evm stream %s: websocket %s disconnected, reconnecting in %s",
			s.chain.Name, evm.RedactEndpoint(s.config.WSEndpoint), backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// subscribeOnce runs one connection. It always returns a non-nil error unless
// ctx is done.
func (s *Stream) subscribeOnce(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	client, err := rpc.DialContext(dialCtx, s.config.WSEndpoint)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	defer client.Close()

	var chainID hexutil.Big
	if err := client.CallContext(dialCtx, &chainID, "eth_chainId"); err != nil {
		return fmt.Errorf("failed to read the chain id: %w", err)
	}

	if id := chainID.ToInt(); !id.IsUint64() || id.Uint64() != uint64(s.chain.ID) {
		return &errWrongChain{expected: s.chain.ID, actual: id.Uint64()}
	}

	heads := make(chan newHead, 16)
	sub, err := client.EthSubscribe(dialCtx, heads, "newHeads")
	if err != nil {
		return fmt.Errorf("failed to subscribe to newHeads: %w", err)
	}

	defer sub.Unsubscribe()

	s.setConnected(true)
	defer s.setConnected(false)

	log.Infof("evm stream %s: subscribed to new heads on %s", s.chain.Name, evm.RedactEndpoint(s.config.WSEndpoint))
	s.EmitConnect()

	err = s.receiveHeads(ctx, heads, sub.Err())
	if ctx.Err() == nil {
		s.EmitDisconnect(err)
	}

	return err
}

func (s *Stream) receiveHeads(ctx context.Context, heads <-chan newHead, subErr <-chan error) error {
	stall := time.NewTimer(s.config.StallTimeout)
	defer stall.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-subErr:
			if err == nil {
				err = errors.New("subscription closed")
			}

			return err

		case <-stall.C:
			return errStalled

		case head := <-heads:
			if head.Number != nil {
				s.observeHead(head.Number.ToInt().Uint64())
			}

			if !stall.Stop() {
				select {
				case <-stall.C:
				default:
				}
			}

			stall.Reset(s.config.StallTimeout)
		}
	}
}
