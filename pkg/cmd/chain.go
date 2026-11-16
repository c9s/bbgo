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
	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

func init() {
	chainCmd.PersistentFlags().String("session", "", "the chain session name")

	chainBalanceCmd.Flags().String("address", "", "the account address to query, defaults to the session address")
	chainBalanceCmd.Flags().StringSlice("tokens", nil,
		"the token symbols or contract addresses to query, defaults to the registered symbols")
	chainBalanceCmd.Flags().Bool("all", false, "also show the tokens with a zero balance")

	chainSendCmd.Flags().String("to", "", "the recipient address")
	chainSendCmd.Flags().String("amount", "", "the amount to send, as a decimal number")
	chainSendCmd.Flags().String("token", "", "the ERC20 token symbol or address; the native currency is sent when omitted")
	chainSendCmd.Flags().Bool("confirm", false, "actually broadcast the transaction instead of only simulating it")

	chainApproveCmd.Flags().String("token", "", "the ERC20 token symbol or address")
	chainApproveCmd.Flags().String("spender", "", "the spender address to approve")
	chainApproveCmd.Flags().String("amount", "", "the allowance to set, as a decimal number")
	chainApproveCmd.Flags().Bool("unlimited", false, "approve the maximum allowance instead of an amount")
	chainApproveCmd.Flags().Bool("confirm", false, "actually broadcast the transaction instead of only simulating it")

	chainAllowanceCmd.Flags().String("token", "", "the ERC20 token symbol or address")
	chainAllowanceCmd.Flags().String("spender", "", "the spender address")

	chainCmd.AddCommand(chainInfoCmd)
	chainCmd.AddCommand(chainBalanceCmd)
	chainCmd.AddCommand(chainSendCmd)
	chainCmd.AddCommand(chainApproveCmd)
	chainCmd.AddCommand(chainAllowanceCmd)
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

// requireSender returns the session sender, which only exists when a private
// key is configured.
func requireSender(session *bbgo.ChainSession) (*evm.Sender, error) {
	sender := session.Sender()
	if sender == nil {
		return nil, fmt.Errorf(
			"chain session %s is read-only: set %s_PRIVATE_KEY (or privateKeyVar) to send transactions",
			session.Name, strings.ToUpper(session.Name))
	}

	return sender, nil
}

// printPlan simulates the request and prints what it would cost. It returns an
// error when the simulation reverts, so a dry run is a real check.
func printPlan(ctx context.Context, session *bbgo.ChainSession, sender *evm.Sender, req evm.SendRequest) error {
	chain := session.EVMChain()

	if _, err := sender.Simulate(ctx, req); err != nil {
		return err
	}

	gasLimit, err := sender.EstimateGas(ctx, req)
	if err != nil {
		return err
	}

	fee, err := sender.EstimateFee(ctx)
	if err != nil {
		return err
	}

	maxCost := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit), fee.Cap())

	fmt.Printf("  FROM:      %s\n", sender.Address().Hex())
	fmt.Printf("  TO:        %s\n", req.To.Hex())
	if req.Value != nil && req.Value.Sign() > 0 {
		fmt.Printf("  VALUE:     %s %s\n", token.FormatRaw(req.Value, chain.NativeDecimals), chain.NativeSymbol)
	}
	fmt.Printf("  GAS LIMIT: %d\n", gasLimit)
	fmt.Printf("  FEE:       %s\n", fee)
	fmt.Printf("  MAX COST:  %s %s\n", token.FormatRaw(maxCost, chain.NativeDecimals), chain.NativeSymbol)

	return nil
}

func printResult(session *bbgo.ChainSession, result *evm.TxResult) {
	chain := session.EVMChain()

	fmt.Printf("  HASH:      %s\n", result.Hash.Hex())
	fmt.Printf("  BLOCK:     %d\n", result.BlockNumber)
	fmt.Printf("  STATUS:    %d\n", result.Status)
	fmt.Printf("  GAS USED:  %d\n", result.GasUsed)
	fmt.Printf("  GAS COST:  %s %s\n", token.FormatRaw(result.GasCost, chain.NativeDecimals), chain.NativeSymbol)
	fmt.Printf("  EXPLORER:  %s\n", chain.TxURL(result.Hash.Hex()))
}

// go run ./cmd/bbgo chain send --session=bsc --to=0x... --amount=0.01 [--token=USDT] [--confirm]
var chainSendCmd = &cobra.Command{
	Use:          "send",
	Short:        "Send the native currency or an ERC20 token (simulates unless --confirm is given)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		sender, err := requireSender(session)
		if err != nil {
			return err
		}

		toFlag, err := cmd.Flags().GetString("to")
		if err != nil {
			return err
		}

		if !common.IsHexAddress(toFlag) {
			return fmt.Errorf("--to must be a valid address, got %q", toFlag)
		}

		to := common.HexToAddress(toFlag)

		amountFlag, err := cmd.Flags().GetString("amount")
		if err != nil {
			return err
		}

		tokenFlag, err := cmd.Flags().GetString("token")
		if err != nil {
			return err
		}

		confirm, err := cmd.Flags().GetBool("confirm")
		if err != nil {
			return err
		}

		chain := session.EVMChain()

		var req evm.SendRequest
		var description string

		if tokenFlag == "" {
			amount, err := token.ParseAmount(amountFlag, chain.NativeDecimals)
			if err != nil {
				return err
			}

			req = evm.SendRequest{To: &to, Value: amount, Label: "send " + chain.NativeSymbol}
			description = fmt.Sprintf("%s %s", token.FormatRaw(amount, chain.NativeDecimals), chain.NativeSymbol)
		} else {
			t, err := session.TokenRegistry().Resolve(ctx, tokenFlag)
			if err != nil {
				return err
			}

			amount, err := token.ParseAmount(amountFlag, t.Decimals)
			if err != nil {
				return err
			}

			data, err := token.TransferCallData(to, amount)
			if err != nil {
				return err
			}

			tokenAddress := t.Address
			req = evm.SendRequest{To: &tokenAddress, Data: data, Label: "transfer " + t.Symbol}
			description = fmt.Sprintf("%s %s", t.FormatRaw(amount), t.Symbol)
		}

		fmt.Printf("SEND %s on %s\n", description, chain.Name)

		if err := printPlan(ctx, session, sender, req); err != nil {
			return err
		}

		if !confirm {
			fmt.Printf("\ndry run only, nothing was sent. pass --confirm to broadcast.\n")
			return nil
		}

		result, err := sender.SendAndWait(ctx, req)
		if result != nil {
			printResult(session, result)
		}

		return err
	},
}

// go run ./cmd/bbgo chain approve --session=bsc --token=USDT --spender=0x... --amount=100 [--confirm]
var chainApproveCmd = &cobra.Command{
	Use:          "approve",
	Short:        "Set an ERC20 allowance (simulates unless --confirm is given)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		sender, err := requireSender(session)
		if err != nil {
			return err
		}

		tokenFlag, err := cmd.Flags().GetString("token")
		if err != nil {
			return err
		}

		spenderFlag, err := cmd.Flags().GetString("spender")
		if err != nil {
			return err
		}

		if !common.IsHexAddress(spenderFlag) {
			return fmt.Errorf("--spender must be a valid address, got %q", spenderFlag)
		}

		spender := common.HexToAddress(spenderFlag)

		amountFlag, err := cmd.Flags().GetString("amount")
		if err != nil {
			return err
		}

		unlimited, err := cmd.Flags().GetBool("unlimited")
		if err != nil {
			return err
		}

		confirm, err := cmd.Flags().GetBool("confirm")
		if err != nil {
			return err
		}

		if unlimited == (amountFlag != "") {
			return fmt.Errorf("pass either --amount or --unlimited")
		}

		t, err := session.TokenRegistry().Resolve(ctx, tokenFlag)
		if err != nil {
			return err
		}

		amount := token.MaxUint256
		if !unlimited {
			amount, err = token.ParseAmount(amountFlag, t.Decimals)
			if err != nil {
				return err
			}
		}

		current, err := token.Allowance(ctx, session.Client(), t.Address, sender.Address(), spender)
		if err != nil {
			return err
		}

		data, err := token.ApproveCallData(spender, amount)
		if err != nil {
			return err
		}

		tokenAddress := t.Address
		req := evm.SendRequest{To: &tokenAddress, Data: data, Label: "approve " + t.Symbol}

		target := t.FormatRaw(amount)
		if unlimited {
			target = "unlimited"
		}

		fmt.Printf("APPROVE %s of %s for %s on %s\n", target, t.Symbol, spender.Hex(), session.EVMChain().Name)
		fmt.Printf("  CURRENT:   %s %s\n", t.FormatRaw(current), t.Symbol)

		if err := printPlan(ctx, session, sender, req); err != nil {
			return err
		}

		if !confirm {
			fmt.Printf("\ndry run only, nothing was sent. pass --confirm to broadcast.\n")
			return nil
		}

		result, err := sender.SendAndWait(ctx, req)
		if result != nil {
			printResult(session, result)
		}

		return err
	},
}

// go run ./cmd/bbgo chain allowance --session=bsc --token=USDT --spender=0x...
var chainAllowanceCmd = &cobra.Command{
	Use:          "allowance",
	Short:        "Show an ERC20 allowance",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, err := selectChainSession(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		tokenFlag, err := cmd.Flags().GetString("token")
		if err != nil {
			return err
		}

		spenderFlag, err := cmd.Flags().GetString("spender")
		if err != nil {
			return err
		}

		if !common.IsHexAddress(spenderFlag) {
			return fmt.Errorf("--spender must be a valid address, got %q", spenderFlag)
		}

		if !session.HasAddress() {
			return fmt.Errorf("chain session %s has no address configured", session.Name)
		}

		t, err := session.TokenRegistry().Resolve(ctx, tokenFlag)
		if err != nil {
			return err
		}

		spender := common.HexToAddress(spenderFlag)

		current, err := token.Allowance(ctx, session.Client(), t.Address, session.EVMAddress(), spender)
		if err != nil {
			return err
		}

		fmt.Printf("%s allowance of %s for %s: %s\n",
			t.Symbol, session.AddressHex(), spender.Hex(), t.FormatRaw(current))

		return nil
	},
}
