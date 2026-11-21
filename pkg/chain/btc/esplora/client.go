// Package esplora reads address UTXOs from an Esplora REST API, the indexer
// behind blockstream.info and mempool.space, which can also be self-hosted
// (Blockstream's electrs fork).
//
// A Bitcoin node keeps no address index; Esplora does. Querying a public
// instance discloses the addresses to its operator.
package esplora

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/chain/btc"
)

const maxResponseSize = 64 << 20

// Client is an Esplora REST client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a client for an Esplora API base URL, such as
// https://mempool.space/api or https://blockstream.info/api.
func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("esplora: invalid base url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("esplora: base url must be http or https, got %q", u.Scheme)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{baseURL: strings.TrimSuffix(baseURL, "/"), httpClient: httpClient}, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("esplora: GET %s: %w", path, err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("esplora: GET %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		// e.g. 400 "Too many unspent transaction outputs" for a busy address
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}

		return nil, fmt.Errorf("esplora: GET %s: http %d: %s", path, resp.StatusCode, msg)
	}

	return body, nil
}

// TipHeight returns the height of the indexer's chain tip.
func (c *Client) TipHeight(ctx context.Context) (int64, error) {
	body, err := c.get(ctx, "/blocks/tip/height")
	if err != nil {
		return 0, err
	}

	height, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("esplora: invalid tip height %q", body)
	}

	return height, nil
}

type utxoStatus struct {
	Confirmed   bool      `json:"confirmed"`
	BlockHeight int64     `json:"block_height"`
	BlockHash   *btc.Hash `json:"block_hash"`
}

type addressUTXO struct {
	TxID   btc.Hash   `json:"txid"`
	Vout   uint32     `json:"vout"`
	Value  int64      `json:"value"` // satoshis
	Status utxoStatus `json:"status"`
}

// AddressUTXOs returns the unspent outputs of an address, including those of
// mempool transactions (Height 0), without the outputs mempool transactions
// spend. Esplora does not report the script or whether an output is from a
// coinbase transaction, so Script is empty and Coinbase false.
func (c *Client) AddressUTXOs(ctx context.Context, address string) ([]btc.UTXO, error) {
	body, err := c.get(ctx, "/address/"+url.PathEscape(address)+"/utxo")
	if err != nil {
		return nil, err
	}

	var raw []addressUTXO
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("esplora: decode the utxos of %s: %w", address, err)
	}

	utxos := make([]btc.UTXO, 0, len(raw))
	for _, r := range raw {
		u := btc.UTXO{
			Outpoint: btc.Outpoint{TxID: r.TxID, Vout: r.Vout},
			Address:  address,
			Value:    btc.Amount(r.Value),
		}

		if r.Status.Confirmed {
			u.Height = r.Status.BlockHeight
			if r.Status.BlockHash != nil {
				u.BlockHash = *r.Status.BlockHash
			}
		}

		utxos = append(utxos, u)
	}

	return utxos, nil
}
