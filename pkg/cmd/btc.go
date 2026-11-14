package cmd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/chain/btc"
	"github.com/c9s/bbgo/pkg/chain/btc/stream"
)

func init() {
	btcCmd.PersistentFlags().String("rpc", "",
		"the Bitcoin Core RPC endpoint, credentials may be in the URL; defaults to $BTC_RPC_ENDPOINT")
	btcCmd.PersistentFlags().String("rpc-cookie", "",
		"the .cookie file of a local bitcoind; defaults to $BTC_RPC_COOKIE")

	btcBlockCmd.Flags().Bool("tx", false, "list the transactions")
	btcBlockCmd.Flags().String("min-value", "", "with --tx, only list transactions moving at least this many BTC")

	btcTxCmd.Flags().String("block", "", "the hash of the block containing the transaction, needed without txindex")

	btcUTXOCmd.Flags().Bool("mempool", true, "take the mempool into account")

	btcStreamCmd.Flags().Int64("from-height", 0, "the first block to print, defaults to the current head")
	btcStreamCmd.Flags().Int64("confirmations", 0, "how many blocks to stay behind the head")
	btcStreamCmd.Flags().Bool("full-tx", false, "read the transactions of every block, with their inputs")
	btcStreamCmd.Flags().String("min-value", "", "with --full-tx, print the transactions moving at least this many BTC")
	btcStreamCmd.Flags().String("chain", "", "the chain the node must follow: main, test, testnet4, signet or regtest")
	btcStreamCmd.Flags().Duration("poll", 0, "the poll interval of the best block hash, 5s by default")

	btcCmd.AddCommand(btcInfoCmd)
	btcCmd.AddCommand(btcBlockCmd)
	btcCmd.AddCommand(btcTxCmd)
	btcCmd.AddCommand(btcUTXOCmd)
	btcCmd.AddCommand(btcScanCmd)
	btcCmd.AddCommand(btcStreamCmd)
	RootCmd.AddCommand(btcCmd)
}

var btcCmd = &cobra.Command{
	Use:          "btc",
	Short:        "Bitcoin node utilities over the Bitcoin Core RPC",
	SilenceUsage: true,
}

func newBTCClient(cmd *cobra.Command) (*btc.Client, error) {
	endpoint, err := cmd.Flags().GetString("rpc")
	if err != nil {
		return nil, err
	}

	if endpoint == "" {
		endpoint = os.Getenv("BTC_RPC_ENDPOINT")
	}

	if endpoint == "" {
		return nil, errors.New("no rpc endpoint, set --rpc or BTC_RPC_ENDPOINT")
	}

	cookie, err := cmd.Flags().GetString("rpc-cookie")
	if err != nil {
		return nil, err
	}

	if cookie == "" {
		cookie = os.Getenv("BTC_RPC_COOKIE")
	}

	var options []btc.RPCOption
	if cookie != "" {
		options = append(options, btc.WithCookieFile(cookie))
	}

	return btc.NewClient(endpoint, options...)
}

func amountFlag(cmd *cobra.Command, name string) (btc.Amount, error) {
	s, err := cmd.Flags().GetString(name)
	if err != nil || s == "" {
		return 0, err
	}

	return btc.ParseAmount(s)
}

// go run ./cmd/bbgo btc info --rpc https://bitcoin-rpc.publicnode.com
var btcInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show the node's chain status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		info, err := client.GetBlockchainInfo(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("CHAIN:      %s\n", info.Chain)
		fmt.Printf("BLOCKS:     %d\n", info.Blocks)
		fmt.Printf("HEADERS:    %d\n", info.Headers)
		fmt.Printf("BEST BLOCK: %s\n", info.BestBlockHash)
		fmt.Printf("TIME:       %s\n", formatUnix(info.Time))
		fmt.Printf("DIFFICULTY: %.0f\n", info.Difficulty)
		fmt.Printf("SUBSIDY:    %s BTC\n", btc.BlockSubsidy(info.Blocks+1))
		fmt.Printf("SYNCED:     %.4f%%, initial download %v\n", info.VerificationProgress*100, info.InitialBlockDownload)
		fmt.Printf("PRUNED:     %v\n", info.Pruned)
		return nil
	},
}

// go run ./cmd/bbgo btc block 975765 --tx --min-value 1
var btcBlockCmd = &cobra.Command{
	Use:   "block <height|hash>",
	Short: "Show a block with its fees and, with --tx, its transactions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		listTx, err := cmd.Flags().GetBool("tx")
		if err != nil {
			return err
		}

		minValue, err := amountFlag(cmd, "min-value")
		if err != nil {
			return err
		}

		hash, err := resolveBlock(ctx, client, args[0])
		if err != nil {
			return err
		}

		block, err := client.GetBlock(ctx, hash)
		if err != nil {
			return err
		}

		printBlockSummary(block)

		if listTx {
			for i := range block.Tx {
				if i == 0 || block.Tx[i].OutputValue() >= minValue {
					printTx(&block.Tx[i])
				}
			}
		}

		return nil
	},
}

// resolveBlock accepts a height or a block hash.
func resolveBlock(ctx context.Context, client *btc.Client, s string) (btc.Hash, error) {
	if height, err := strconv.ParseInt(s, 10, 64); err == nil && len(s) < btc.HashSize*2 {
		return client.GetBlockHash(ctx, height)
	}

	return btc.NewHashFromString(s)
}

// go run ./cmd/bbgo btc tx <txid> --block <hash>
var btcTxCmd = &cobra.Command{
	Use:   "tx <txid>",
	Short: "Show a transaction with the outputs its inputs spend",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		txid, err := btc.NewHashFromString(args[0])
		if err != nil {
			return err
		}

		var blockHash *btc.Hash
		if s, _ := cmd.Flags().GetString("block"); s != "" {
			h, err := btc.NewHashFromString(s)
			if err != nil {
				return err
			}

			blockHash = &h
		}

		tx, err := client.GetRawTransaction(ctx, txid, blockHash)
		if err != nil {
			if btc.IsNotFound(err) && blockHash == nil {
				return fmt.Errorf("%w (without txindex the node only finds a confirmed transaction with --block)", err)
			}

			return err
		}

		if tx.BlockHash != nil {
			fmt.Printf("BLOCK: %s, %d confirmations, %s\n", tx.BlockHash, tx.Confirmations, formatUnix(tx.BlockTime))
		} else {
			fmt.Printf("BLOCK: (mempool)\n")
		}

		printTx(tx)
		return nil
	},
}

// go run ./cmd/bbgo btc utxo <txid>:<vout>
var btcUTXOCmd = &cobra.Command{
	Use:   "utxo <txid:vout>",
	Short: "Show an output if it is unspent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		outpoint, err := btc.ParseOutpoint(args[0])
		if err != nil {
			return err
		}

		mempool, err := cmd.Flags().GetBool("mempool")
		if err != nil {
			return err
		}

		out, err := client.GetTxOut(ctx, outpoint, mempool)
		if errors.Is(err, btc.ErrNotFound) {
			fmt.Printf("%s is spent or does not exist\n", outpoint)
			return nil
		} else if err != nil {
			return err
		}

		fmt.Printf("OUTPOINT:      %s\n", outpoint)
		fmt.Printf("VALUE:         %s BTC\n", out.Value)
		fmt.Printf("ADDRESS:       %s\n", scriptLabel(&out.ScriptPubKey))
		fmt.Printf("TYPE:          %s\n", out.ScriptPubKey.Type)
		fmt.Printf("CONFIRMATIONS: %d\n", out.Confirmations)
		fmt.Printf("COINBASE:      %v\n", out.Coinbase)
		return nil
	},
}

// go run ./cmd/bbgo btc scan bc1q... 'wpkh(xpub.../0/*)'
var btcScanCmd = &cobra.Command{
	Use:   "scan <address|descriptor>...",
	Short: "Find the UTXOs of addresses or descriptors by scanning the whole UTXO set",
	Long: "Find the UTXOs of addresses or descriptors with scantxoutset. The scan walks the whole UTXO set " +
		"and takes tens of seconds on mainnet; public providers usually disable it.",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		descriptors := make([]string, len(args))
		for i, a := range args {
			descriptors[i] = a
			if !strings.Contains(a, "(") {
				descriptors[i] = "addr(" + a + ")"
			}
		}

		result, err := client.ScanTxOutSet(ctx, descriptors)
		if ctx.Err() != nil {
			// the node keeps scanning after the request is gone
			if _, abortErr := client.AbortScanTxOutSet(context.Background()); abortErr != nil {
				return abortErr
			}
		}

		if err != nil {
			return err
		}

		fmt.Printf("HEIGHT: %d (%s), %d outputs scanned\n", result.Height, result.BestBlock, result.TxOuts)
		for _, u := range result.Unspents {
			fmt.Printf("  %s %16s BTC height=%d coinbase=%v %s\n", u.Outpoint(), u.Amount, u.Height, u.Coinbase, u.Desc)
		}

		fmt.Printf("TOTAL: %s BTC in %d outputs\n", result.TotalAmount, len(result.Unspents))
		return nil
	},
}

// go run ./cmd/bbgo btc stream --full-tx --min-value 10
var btcStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Follow the chain block by block, reporting reorgs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client, err := newBTCClient(cmd)
		if err != nil {
			return err
		}

		flags := cmd.Flags()

		var config stream.Config
		if config.FromHeight, err = flags.GetInt64("from-height"); err != nil {
			return err
		}

		if config.Confirmations, err = flags.GetInt64("confirmations"); err != nil {
			return err
		}

		if config.FullTransactions, err = flags.GetBool("full-tx"); err != nil {
			return err
		}

		if config.Chain, err = flags.GetString("chain"); err != nil {
			return err
		}

		if config.PollInterval, err = flags.GetDuration("poll"); err != nil {
			return err
		}

		minValue, err := amountFlag(cmd, "min-value")
		if err != nil {
			return err
		}

		s := stream.New(client, config)

		s.OnBlock(func(b *stream.Block) {
			if !config.FullTransactions {
				fmt.Printf("block %d %s %s txs=%d\n", b.Height, b.Hash, formatUnix(b.Time), len(b.TxIDs))
				return
			}

			printBlockSummary(b.Block)

			if minValue > 0 {
				for i := 1; i < len(b.Tx); i++ {
					if b.Tx[i].OutputValue() >= minValue {
						printTx(&b.Tx[i])
					}
				}
			}
		})

		s.OnReorg(func(r stream.Reorg) {
			removed := make([]string, 0, len(r.Removed))
			for _, ref := range r.Removed {
				removed = append(removed, ref.String())
			}

			fmt.Printf("reorg: back to %s, removed %s\n", r.Ancestor, strings.Join(removed, ", "))
		})

		err = s.Run(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return err
	},
}

func printBlockSummary(b *btc.Block) {
	fmt.Printf("block %d %s %s txs=%d weight=%d\n", b.Height, b.Hash, formatUnix(b.Time), len(b.Tx), b.Weight)

	if len(b.Tx) == 0 {
		return
	}

	var fees btc.Amount
	feesKnown := true
	for i := 1; i < len(b.Tx); i++ {
		if b.Tx[i].Fee == nil {
			feesKnown = false
			break
		}

		fees += *b.Tx[i].Fee
	}

	coinbase := &b.Tx[0]
	fmt.Printf("  coinbase pays %s BTC (subsidy %s", coinbase.OutputValue(), btc.BlockSubsidy(b.Height))
	if feesKnown {
		fmt.Printf(", fees %s", fees)
	}
	fmt.Printf(")")

	if len(coinbase.Vin) > 0 {
		if tag := coinbaseTag(coinbase.Vin[0].Coinbase); tag != "" {
			fmt.Printf(" tag %q", tag)
		}
	}

	fmt.Println()
}

func printTx(tx *btc.Tx) {
	fee := "?"
	if tx.Fee != nil {
		fee = tx.Fee.String()
	}

	if tx.IsCoinbase() {
		fee = "(coinbase)"
	}

	fmt.Printf("  tx %s vsize=%d fee=%s out=%s BTC\n", tx.TxID, tx.VSize, fee, tx.OutputValue())

	for i := range tx.Vin {
		in := &tx.Vin[i]
		switch {
		case in.IsCoinbase():
			fmt.Printf("    in  coinbase %s\n", in.Coinbase)
		case in.Prevout != nil:
			fmt.Printf("    in  %16s %s (%s)\n", in.Prevout.Value, scriptLabel(&in.Prevout.ScriptPubKey), in.Outpoint())
		default:
			fmt.Printf("    in  %16s %s\n", "?", in.Outpoint())
		}
	}

	for i := range tx.Vout {
		out := &tx.Vout[i]
		fmt.Printf("    out %16s %s\n", out.Value, scriptLabel(&out.ScriptPubKey))
	}
}

// scriptLabel is the address of a script, or its type when it has none.
func scriptLabel(s *btc.ScriptPubKey) string {
	if s.Address != "" {
		return s.Address
	}

	if s.Type == btc.ScriptTypeNullData {
		return "OP_RETURN " + strings.TrimPrefix(s.Asm, "OP_RETURN ")
	}

	return "(" + string(s.Type) + ")"
}

// coinbaseTag extracts the readable runs of the coinbase field, where
// mining pools write their name ("/Foundry USA Pool/").
func coinbaseTag(coinbaseHex string) string {
	data, err := hex.DecodeString(coinbaseHex)
	if err != nil {
		return ""
	}

	var runs []string
	start := -1
	for i := 0; i <= len(data); i++ {
		printable := i < len(data) && data[i] >= 0x20 && data[i] < 0x7f
		if printable && start < 0 {
			start = i
		} else if !printable && start >= 0 {
			if i-start >= 4 {
				runs = append(runs, string(data[start:i]))
			}

			start = -1
		}
	}

	return strings.Join(runs, " ")
}

func formatUnix(sec int64) string {
	if sec == 0 {
		return "-"
	}

	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
