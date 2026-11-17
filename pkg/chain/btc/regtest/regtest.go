// Package regtest runs a throwaway bitcoind in regtest mode for integration
// tests: a private chain where blocks are mined on demand, so coinbase
// maturity, reorgs and wallet transactions can be produced exactly.
//
// Start skips the test when no bitcoind binary is found. Point $BITCOIND at
// one, or put it on $PATH.
package regtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

// Node is a running regtest bitcoind with a loaded wallet.
type Node struct {
	*btc.Client

	// DataDir is the node's data directory, removed when the test ends.
	DataDir string

	// RPCURL is the RPC endpoint; the cookie file authenticates it.
	RPCURL     string
	CookieFile string

	cmd    *exec.Cmd
	stderr bytes.Buffer
	exited chan struct{}
}

// Binary returns the bitcoind to run: $BITCOIND, or bitcoind on $PATH.
func Binary() (string, error) {
	if p := os.Getenv("BITCOIND"); p != "" {
		return p, nil
	}

	return exec.LookPath("bitcoind")
}

// Start runs a fresh regtest node with an empty wallet named "test", and
// stops it when the test ends. It skips the test when bitcoind is missing.
func Start(t testing.TB, extraArgs ...string) *Node {
	t.Helper()

	bin, err := Binary()
	if err != nil {
		t.Skip("bitcoind not found; set BITCOIND to run the regtest tests")
	}

	n, err := start(bin, t.TempDir(), extraArgs)
	if err != nil {
		t.Fatalf("regtest: %v", err)
	}

	t.Cleanup(n.stop)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := n.Call(ctx, nil, "createwallet", "test"); err != nil {
		t.Fatalf("regtest: createwallet: %v", err)
	}

	return n
}

func start(bin, dataDir string, extraArgs []string) (*Node, error) {
	rpcPort, err := freePort()
	if err != nil {
		return nil, err
	}

	args := append([]string{
		"-regtest",
		"-datadir=" + dataDir,
		"-server",
		"-listen=0",
		"-dnsseed=0",
		"-fixedseeds=0",
		"-rpcbind=127.0.0.1",
		"-rpcallowip=127.0.0.1",
		"-rpcport=" + strconv.Itoa(rpcPort),
		"-fallbackfee=0.0001",
		"-printtoconsole=0",
	}, extraArgs...)

	n := &Node{
		DataDir:    dataDir,
		RPCURL:     fmt.Sprintf("http://127.0.0.1:%d", rpcPort),
		CookieFile: filepath.Join(dataDir, "regtest", ".cookie"),
		exited:     make(chan struct{}),
	}

	n.cmd = exec.Command(bin, args...)
	n.cmd.Stderr = &n.stderr
	if err := n.cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		_ = n.cmd.Wait()
		close(n.exited)
	}()

	if n.Client, err = btc.NewClient(n.RPCURL, btc.WithCookieFile(n.CookieFile)); err != nil {
		n.stop()
		return nil, err
	}

	if err := n.waitReady(30 * time.Second); err != nil {
		n.stop()
		return nil, err
	}

	return n, nil
}

func (n *Node) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-n.exited:
			return fmt.Errorf("bitcoind exited: %s", n.stderr.String())
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := n.GetBlockCount(ctx)
		cancel()
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("bitcoind not ready after %s: %w", timeout, err)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (n *Node) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = n.Call(ctx, nil, "stop")

	select {
	case <-n.exited:
	case <-time.After(30 * time.Second):
		_ = n.cmd.Process.Kill()
		<-n.exited
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// NewAddress returns a new bech32 receive address of the wallet.
func (n *Node) NewAddress(ctx context.Context) (string, error) {
	var address string
	err := n.Call(ctx, &address, "getnewaddress", "", "bech32")
	return address, err
}

// Mine mines count blocks paying their coinbase to address, including the
// mempool, and returns their hashes.
func (n *Node) Mine(ctx context.Context, count int, address string) ([]btc.Hash, error) {
	var hashes []btc.Hash
	err := n.Call(ctx, &hashes, "generatetoaddress", count, address)
	return hashes, err
}

// Send pays amount to address from the wallet and returns the txid. It stays
// in the mempool until the next block is mined.
func (n *Node) Send(ctx context.Context, address string, amount btc.Amount) (btc.Hash, error) {
	var txid btc.Hash
	err := n.Call(ctx, &txid, "sendtoaddress", address, amount.String())
	return txid, err
}

// SpendAll spends exactly the given outputs to address, the fee taken from
// the amount, and returns the txid.
func (n *Node) SpendAll(ctx context.Context, address string, inputs ...btc.Outpoint) (btc.Hash, error) {
	ins := make([]map[string]any, len(inputs))
	for i, o := range inputs {
		ins[i] = map[string]any{"txid": o.TxID.String(), "vout": o.Vout}
	}

	var result struct {
		TxID     btc.Hash `json:"txid"`
		Complete bool     `json:"complete"`
	}

	if err := n.Call(ctx, &result, "sendall", []string{address}, nil, nil, nil,
		map[string]any{"inputs": ins}); err != nil {
		return btc.Hash{}, err
	}

	if !result.Complete {
		return btc.Hash{}, errors.New("sendall: transaction not complete")
	}

	return result.TxID, nil
}

// InvalidateBlock marks a block invalid; the node falls back to its parent.
func (n *Node) InvalidateBlock(ctx context.Context, hash btc.Hash) error {
	return n.Call(ctx, nil, "invalidateblock", hash.String())
}
