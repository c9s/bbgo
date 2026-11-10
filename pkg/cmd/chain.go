package cmd

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

func init() {
	chainCmd.PersistentFlags().String("session", "", "the chain session name")

	chainBalanceCmd.Flags().String("address", "", "the account address to query, defaults to the session address")
	chainBalanceCmd.Flags().StringSlice("tokens", nil,
		"the token symbols or contract addresses to query, defaults to the registered symbols")
	chainBalanceCmd.Flags().Bool("all", false, "also show the tokens with a zero balance")

	chainCmd.AddCommand(chainInfoCmd)
	chainCmd.AddCommand(chainBalanceCmd)
	RootCmd.AddCommand(chainCmd)
}

var chainCmd = &cobra.Command{
	Use:          "chain",
	Short:        "On-chain (EVM) utilities",
	SilenceUsage: true,
}

// selectChainSession configures the chain sessions and returns the one selected
// by --session. The flag may be omitted when exactly one session is defined.
func selectChainSession(ctx context.Context, cmd *cobra.Command) (*bbgo.Environment, *bbgo.ChainSession, error) {
	sessionName, err := cmd.Flags().GetString("session")
	if err != nil {
		return nil, nil, err
	}

	environ := bbgo.NewEnvironment()

	if err := environ.ConfigureChainSessions(ctx, userConfig); err != nil {
		return nil, nil, err
	}

	sessions := environ.ChainSessions()
	if len(sessions) == 0 {
		return nil, nil, fmt.Errorf("no chain session is defined, add a chainSessions section to the config file")
	}

	if sessionName == "" {
		if len(sessions) > 1 {
			return nil, nil, fmt.Errorf("--session is required, defined chain sessions: %s",
				strings.Join(sessions.Names(), ", "))
		}

		sessionName = sessions.Names()[0]
	}

	session, ok := environ.ChainSession(sessionName)
	if !ok {
		return nil, nil, fmt.Errorf("chain session %s not found, defined chain sessions: %s",
			sessionName, strings.Join(sessions.Names(), ", "))
	}

	return environ, session, nil
}

// go run ./cmd/bbgo chain info --session=bsc
var chainInfoCmd = &cobra.Command{
	Use:          "info",
	Short:        "Show the chain session status",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		chain := session.EVMChain()
		client := session.Client()

		blockNumber, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("SESSION:       %s\n", session.Name)
		fmt.Printf("CHAIN:         %s (chain id %d)\n", chain.Name, uint64(chain.ID))
		fmt.Printf("NATIVE:        %s\n", chain.NativeSymbol)
		fmt.Printf("FEE MARKET:    %s\n", chain.FeeMarket)
		fmt.Printf("BLOCK:         %d\n", blockNumber)
		fmt.Printf("CONFIRMATIONS: %d\n", session.Confirmations)
		fmt.Printf("ADDRESS:       %s\n", session.AddressHex())
		fmt.Printf("CAN SIGN:      %v\n", session.CanSign())
		fmt.Printf("ENDPOINTS:     %s\n", strings.Join(client.Endpoints(), ", "))
		fmt.Printf("TOKENS:        %s\n", strings.Join(session.TokenRegistry().Symbols(), ", "))

		return nil
	},
}

// go run ./cmd/bbgo chain balance --session=bsc --tokens=BTCB,USDT
var chainBalanceCmd = &cobra.Command{
	Use:          "balance",
	Short:        "Show the native and ERC20 balances of a chain session",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		addressFlag, err := cmd.Flags().GetString("address")
		if err != nil {
			return err
		}

		tokenRefs, err := cmd.Flags().GetStringSlice("tokens")
		if err != nil {
			return err
		}

		showAll, err := cmd.Flags().GetBool("all")
		if err != nil {
			return err
		}

		address := session.EVMAddress()
		if addressFlag != "" {
			if !common.IsHexAddress(addressFlag) {
				return fmt.Errorf("invalid address %q", addressFlag)
			}

			address = common.HexToAddress(addressFlag)
		}

		if address == (common.Address{}) {
			return fmt.Errorf("no address to query: pass --address, or configure an address or a private key on the chain session")
		}

		chain := session.EVMChain()

		nativeBalance, err := session.Client().NativeBalance(ctx, address)
		if err != nil {
			return err
		}

		fmt.Printf("CHAIN %s ADDRESS %s\n", chain.Name, address.Hex())
		fmt.Printf("  %-10s %s\n", chain.NativeSymbol, token.FormatRaw(nativeBalance, chain.NativeDecimals))

		registry := session.TokenRegistry()

		if len(tokenRefs) == 0 {
			tokenRefs = registry.Symbols()
		}

		if len(tokenRefs) == 0 {
			return nil
		}

		tokens, err := registry.ResolveAll(ctx, tokenRefs)
		if err != nil {
			return err
		}

		balances, err := registry.Balances(ctx, address, tokens)
		if err != nil {
			return err
		}

		sort.Slice(balances, func(i, j int) bool {
			return balances[i].Token.Symbol < balances[j].Token.Symbol
		})

		for _, balance := range balances {
			if !showAll && balance.Raw.Cmp(big.NewInt(0)) == 0 {
				continue
			}

			fmt.Printf("  %-10s %-30s %s\n",
				balance.Token.Symbol, balance.Token.FormatRaw(balance.Raw), balance.Token.Address.Hex())
		}

		return nil
	},
}
