package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/stream"
	"github.com/c9s/bbgo/pkg/chain/btc/watch"
)

// go run ./cmd/bbgo btc watch bc1q... 1A1z... --confirmations 1
var btcWatchCmd = &cobra.Command{
	Use:   "watch <address>...",
	Short: "Watch addresses: print the outputs they receive and spend, and their balances",
	Long: "Watch addresses block by block. Without --seed, only outputs received after the watch starts " +
		"count towards the balances; spends of older outputs are still reported, as not tracked.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		flags := cmd.Flags()

		config := stream.Config{FullTransactions: true}
		if config.FromHeight, err = flags.GetInt64("from-height"); err != nil {
			return err
		}

		if config.Confirmations, err = flags.GetInt64("confirmations"); err != nil {
			return err
		}

		if config.Chain, err = flags.GetString("chain"); err != nil {
			return err
		}

		if config.PollInterval, err = flags.GetDuration("poll"); err != nil {
			return err
		}

		seed, err := flags.GetBool("seed")
		if err != nil {
			return err
		}

		w := watch.New(watch.Config{Addresses: args})

		if seed {
			if config.FromHeight > 0 {
				return errors.New("--seed starts after the scanned height; it cannot be used with --from-height")
			}

			descriptors := make([]string, len(args))
			for i, a := range args {
				descriptors[i] = "addr(" + a + ")"
			}

			fmt.Printf("scanning the UTXO set for %d address(es)...\n", len(args))

			result, err := client.ScanTxOutSet(ctx, descriptors)
			if err != nil {
				return fmt.Errorf("scantxoutset: %w", err)
			}

			utxos := make([]watch.UTXO, 0, len(result.Unspents))
			for _, u := range result.Unspents {
				utxo := watch.UTXO{
					Outpoint: u.Outpoint(),
					Address:  descriptorAddress(u.Desc),
					Script:   u.ScriptPubKey,
					Value:    u.Amount,
					Height:   u.Height,
					Coinbase: u.Coinbase,
				}

				if u.BlockHash != nil {
					utxo.BlockHash = *u.BlockHash
				}

				utxos = append(utxos, utxo)
			}

			w.Seed(utxos)

			// the UTXO set is as of result.Height; the stream picks up from the next block
			config.FromHeight = result.Height + 1

			fmt.Printf("seeded %d UTXO(s) at height %d\n", len(utxos), result.Height)
			printBalances(w, args)
		}

		s := stream.New(client, config)
		if err := w.Attach(s); err != nil {
			return err
		}

		w.OnEvent(func(e watch.Event) {
			note := ""
			if e.Kind == watch.Spent && !e.Known {
				note = " (received before the watch started, not tracked)"
			}

			fmt.Printf("%s%s\n", e.String(), note)
		})

		w.OnRevert(func(e watch.Event) {
			fmt.Printf("reverted: %s\n", e.String())
		})

		changed := false
		w.OnEvent(func(watch.Event) { changed = true })
		w.OnRevert(func(watch.Event) { changed = true })

		// registered after Attach, so the watcher has applied the block already
		s.OnBlock(func(b *stream.Block) {
			fmt.Printf("block %d %s txs=%d\n", b.Height, b.Hash, len(b.Tx))
			if changed {
				printBalances(w, args)
				changed = false
			}
		})

		s.OnReorg(func(r stream.Reorg) {
			fmt.Printf("reorg: back to %s, %d block(s) removed\n", r.Ancestor, len(r.Removed))
			if changed {
				printBalances(w, args)
				changed = false
			}
		})

		err = s.Run(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	},
}

// descriptorAddress returns X of "addr(X)#checksum", or "" for another descriptor.
func descriptorAddress(desc string) string {
	desc, _, _ = strings.Cut(desc, "#")
	if !strings.HasPrefix(desc, "addr(") || !strings.HasSuffix(desc, ")") {
		return ""
	}

	return desc[len("addr(") : len(desc)-1]
}

func printBalances(w *watch.Watcher, addresses []string) {
	balances := w.Balances()

	keys := make([]string, 0, len(balances))
	for k := range balances {
		keys = append(keys, k)
	}

	// the watched addresses first, in the order given, even at zero
	seen := map[string]bool{}
	var order []string
	for _, a := range addresses {
		k := watch.NormalizeAddress(a)
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
	}

	sort.Strings(keys)
	for _, k := range keys {
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
	}

	var total btc.Amount
	for _, k := range order {
		total += balances[k]
		fmt.Printf("  balance %16s BTC %s\n", balances[k], k)
	}

	fmt.Printf("  total   %16s BTC in %d UTXO(s)\n", total, len(w.UTXOs()))
}
