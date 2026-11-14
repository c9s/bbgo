package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/chain/btc/balance"
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

			fmt.Printf("scanning the UTXO set for %d address(es)...\n", len(args))

			snapshot, err := (&balance.CoreSource{Client: client}).Snapshot(ctx, args)
			if err != nil {
				return fmt.Errorf("scantxoutset: %w", err)
			}

			w.Seed(snapshot.UTXOs)

			// the UTXO set is as of snapshot.Height; the stream picks up from the next block
			config.FromHeight = snapshot.Height + 1

			fmt.Printf("seeded %d UTXO(s) at height %d\n", len(snapshot.UTXOs), snapshot.Height)
			printBalanceTable(balance.Compute(snapshot, 1), args)
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
				printBalanceTable(balance.Compute(w.Snapshot(), 1), args)
				changed = false
			}
		})

		s.OnReorg(func(r stream.Reorg) {
			fmt.Printf("reorg: back to %s, %d block(s) removed\n", r.Ancestor, len(r.Removed))
			if changed {
				printBalanceTable(balance.Compute(w.Snapshot(), 1), args)
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
