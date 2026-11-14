package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/balance"
	"github.com/c9s/bbgo/pkg/chain/btc/esplora"
)

func init() {
	btcBalanceCmd.Flags().String("esplora", "",
		"read the UTXOs from this Esplora API (e.g. https://mempool.space/api) instead of scantxoutset; "+
			"defaults to $ESPLORA_URL. A public instance learns the addresses")
	btcBalanceCmd.Flags().Int64("min-conf", 1, "the confirmations an output needs to count as confirmed")
	btcBalanceCmd.Flags().Bool("utxos", false, "list the UTXOs")

	btcCmd.AddCommand(btcBalanceCmd)
}

// go run ./cmd/bbgo btc balance bc1q... --esplora https://mempool.space/api
var btcBalanceCmd = &cobra.Command{
	Use:   "balance <address|descriptor>...",
	Short: "Compute the balances of addresses from their UTXOs",
	Long: "Compute the balances of addresses from their UTXOs, split into confirmed, pending (fewer confirmations " +
		"than --min-conf, or in the mempool) and immature coinbase outputs. The UTXOs come from the node's " +
		"scantxoutset (your own node; it scans the whole UTXO set and sees no mempool) or from an Esplora indexer.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		flags := cmd.Flags()

		esploraURL, err := flags.GetString("esplora")
		if err != nil {
			return err
		}

		if esploraURL == "" {
			esploraURL = os.Getenv("ESPLORA_URL")
		}

		minConf, err := flags.GetInt64("min-conf")
		if err != nil {
			return err
		}

		listUTXOs, err := flags.GetBool("utxos")
		if err != nil {
			return err
		}

		var source balance.Source
		if esploraURL != "" {
			client, err := esplora.NewClient(esploraURL, nil)
			if err != nil {
				return err
			}

			source = &balance.EsploraSource{Client: client}
		} else {
			client, err := newBTCClient(cmd)
			if err != nil {
				return fmt.Errorf("%w (or use --esplora)", err)
			}

			fmt.Printf("scanning the UTXO set...\n")
			source = &balance.CoreSource{Client: client}
		}

		snapshot, err := source.Snapshot(ctx, args)
		if err != nil {
			return err
		}

		fmt.Printf("as of block %d, confirmed with >= %d confirmation(s)\n", snapshot.Height, minConf)

		if listUTXOs {
			for i := range snapshot.UTXOs {
				printUTXO(&snapshot.UTXOs[i], snapshot.Height)
			}
		}

		printBalanceTable(balance.Compute(snapshot, minConf), args)
		return nil
	},
}

func printUTXO(u *btc.UTXO, tip int64) {
	state := fmt.Sprintf("%d conf", u.Confirmations(tip))
	if !u.Confirmed() {
		state = "mempool"
	}

	if !u.Mature(tip) {
		state += ", immature coinbase"
	}

	label := u.Address
	if label == "" {
		label = u.Script
	}

	fmt.Printf("  utxo %s %16s BTC %s (%s)\n", u.Outpoint, u.Value, label, state)
}

// printBalanceTable prints the given addresses first, in order and even at
// zero, then any other key of the balances (scripts, descriptor matches).
func printBalanceTable(balances []balance.Balance, addresses []string) {
	fmt.Printf("  %-62s %16s %16s %16s %6s\n", "ADDRESS", "CONFIRMED", "PENDING", "IMMATURE", "UTXOS")

	printed := map[string]bool{}
	row := func(b balance.Balance) {
		printed[b.Address] = true
		fmt.Printf("  %-62s %16s %16s %16s %6d\n", b.Address, b.Confirmed, b.Pending, b.Immature, b.UTXOs)
	}

	for _, a := range addresses {
		// a descriptor matches many addresses; they are listed below
		if strings.Contains(a, "(") {
			continue
		}

		if b := balance.Of(balances, a); !printed[b.Address] {
			row(b)
		}
	}

	for _, b := range balances {
		if !printed[b.Address] {
			row(b)
		}
	}

	total := balance.Sum(balances)
	fmt.Printf("  %-62s %16s %16s %16s %6d\n", "TOTAL", total.Confirmed, total.Pending, total.Immature, total.UTXOs)
}
