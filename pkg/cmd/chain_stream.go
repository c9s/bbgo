package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/chain/evm/stream"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

func init() {
	chainStreamCmd.Flags().Uint64("from-block", 0, "the first block to print, defaults to the current head")
	chainStreamCmd.Flags().Int("confirmations", 0, "how many blocks to stay behind the head")
	chainStreamCmd.Flags().Bool("full-tx", false, "fetch the transactions of every block")
	chainStreamCmd.Flags().String("ws", "", "the websocket endpoint, overriding the session's")
	chainStreamCmd.Flags().StringSlice("address", nil,
		"fetch the logs of these contracts, as token symbols or addresses")
	// a string array, not a slice: an event signature contains commas
	chainStreamCmd.Flags().StringArray("event", nil,
		"fetch only these events, as signatures like Transfer(address,address,uint256) or topic hashes; repeat for several")

	chainCmd.AddCommand(chainStreamCmd)
}

// go run ./cmd/bbgo chain stream --session=bsc --address=USDT --event='Transfer(address,address,uint256)'
var chainStreamCmd = &cobra.Command{
	Use:          "stream",
	Short:        "Follow the chain block by block, with optional log filtering",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		flags := cmd.Flags()

		var config stream.Config
		if config.FromBlock, err = flags.GetUint64("from-block"); err != nil {
			return err
		}

		if config.Confirmations, err = flags.GetInt("confirmations"); err != nil {
			return err
		}

		if config.FullTransactions, err = flags.GetBool("full-tx"); err != nil {
			return err
		}

		if config.WSEndpoint, err = flags.GetString("ws"); err != nil {
			return err
		}

		addressFlags, err := flags.GetStringSlice("address")
		if err != nil {
			return err
		}

		eventFlags, err := flags.GetStringArray("event")
		if err != nil {
			return err
		}

		if len(addressFlags) > 0 || len(eventFlags) > 0 {
			filter := &stream.LogFilter{}

			for _, a := range addressFlags {
				if common.IsHexAddress(a) {
					filter.Addresses = append(filter.Addresses, common.HexToAddress(a))
					continue
				}

				t, err := session.TokenRegistry().Resolve(ctx, a)
				if err != nil {
					return err
				}

				filter.Addresses = append(filter.Addresses, t.Address)
			}

			if len(eventFlags) > 0 {
				topics := make([]common.Hash, 0, len(eventFlags))
				for _, e := range eventFlags {
					topics = append(topics, parseEventTopic(e))
				}

				filter.Topics = [][]common.Hash{topics}
			}

			config.Logs = filter
		}

		s, err := session.NewStream(config)
		if err != nil {
			return err
		}

		chain := session.EVMChain()

		s.OnConnect(func() {
			fmt.Printf("websocket connected\n")
		})

		s.OnDisconnect(func(err error) {
			fmt.Printf("websocket disconnected: %v\n", err)
		})

		s.OnBlock(func(b *stream.Block) {
			fmt.Printf("block %d %s %s txs=%d logs=%d\n",
				b.Number(), b.Hash.Hex(),
				time.Unix(int64(b.Time()), 0).Format(time.RFC3339),
				b.TxCount(), len(b.Logs))

			for _, tx := range b.Transactions {
				to := "(create)"
				if tx.To() != nil {
					to = tx.To().Hex()
				}

				fmt.Printf("  tx %s %s -> %s value=%s %s\n",
					tx.Hash().Hex(), tx.From.Hex(), to, token.FormatRaw(tx.Value(), chain.NativeDecimals), chain.NativeSymbol)
			}

			if b.SkippedTransactions > 0 {
				fmt.Printf("  (%d transactions of an unsupported type skipped)\n", b.SkippedTransactions)
			}
		})

		s.OnLog(func(l ethtypes.Log) {
			topic0 := "(anonymous)"
			if len(l.Topics) > 0 {
				topic0 = l.Topics[0].Hex()
			}

			fmt.Printf("  log %s#%d %s topic0=%s\n", l.TxHash.Hex(), l.Index, l.Address.Hex(), topic0)
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

// parseEventTopic accepts an event signature or a raw topic hash.
func parseEventTopic(s string) common.Hash {
	if strings.HasPrefix(s, "0x") && len(s) == 66 {
		return common.HexToHash(s)
	}

	return crypto.Keccak256Hash([]byte(strings.ReplaceAll(s, " ", "")))
}
