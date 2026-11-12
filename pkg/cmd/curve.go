package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
	"github.com/c9s/bbgo/pkg/defi/curve"
)

func init() {
	curveCmd.PersistentFlags().String("session", "", "the chain session name")

	curvePoolsCmd.Flags().String("base", "", "the base coin symbol or address")
	curvePoolsCmd.Flags().String("quote", "", "the quote coin symbol or address")

	curvePoolCmd.Flags().String("address", "", "the pool address")

	curveQuoteCmd.Flags().String("from", "", "the input coin symbol or address")
	curveQuoteCmd.Flags().String("to", "", "the output coin symbol or address")
	curveQuoteCmd.Flags().String("amount", "", "the input amount, as a decimal number")

	curveSwapCmd.Flags().String("from", "", "the input coin symbol or address")
	curveSwapCmd.Flags().String("to", "", "the output coin symbol or address")
	curveSwapCmd.Flags().String("amount", "", "the input amount, as a decimal number")
	curveSwapCmd.Flags().Int("slippage", 50, "the slippage tolerance in basis points")
	curveSwapCmd.Flags().String("pool", "", "the pool to swap through; the best quote is used when omitted")
	curveSwapCmd.Flags().Bool("confirm", false, "actually send the swap")

	curveCmd.AddCommand(curvePoolsCmd)
	curveCmd.AddCommand(curvePoolCmd)
	curveCmd.AddCommand(curveQuoteCmd)
	curveCmd.AddCommand(curveSwapCmd)
	RootCmd.AddCommand(curveCmd)
}

var curveCmd = &cobra.Command{
	Use:          "curve",
	Short:        "Curve utilities: pools, quotes and swaps",
	SilenceUsage: true,
}

func newCurveClient(
	ctx context.Context, cmd *cobra.Command,
) (*bbgo.Environment, *bbgo.ChainSession, *curve.Client, error) {
	environ, session, err := selectChainSession(ctx, cmd)
	if err != nil {
		return nil, nil, nil, err
	}

	options := curve.Options{
		Tokens:    session.TokenRegistry(),
		Allowance: session.AllowanceConfig(),
	}

	if sender := session.Sender(); sender != nil {
		options.Sender = sender
	}

	client, err := curve.NewClient(ctx, session.EVMChain(), session.Client(), options)
	if err != nil {
		environ.CloseChainSessions()
		return nil, nil, nil, err
	}

	return environ, session, client, nil
}

// go run ./cmd/bbgo curve pools --session=ethereum --base=USDC --quote=USDT
var curvePoolsCmd = &cobra.Command{
	Use:          "pools",
	Short:        "List the pools that trade a pair, deepest first",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newCurveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		base, quote, err := resolvePair(ctx, cmd, session, "base", "quote")
		if err != nil {
			return err
		}

		routes, err := client.Routes(ctx, base.Address, quote.Address)
		if err != nil {
			return err
		}

		fmt.Printf("CURVE %s/%s on %s, metaregistry %s\n",
			base.Symbol, quote.Symbol, client.ChainName(), client.MetaRegistryAddress().Hex())
		fmt.Printf("  %-44s %-20s %8s %5s %5s %26s\n",
			"POOL", "NAME", "FEE", "I", "J", strings.ToUpper(quote.Symbol)+" HELD")

		addresses := make([]common.Address, 0, len(routes))
		for _, route := range routes {
			addresses = append(addresses, route.Pool)
		}

		pools, err := client.Pools(ctx, addresses)
		if err != nil {
			return err
		}

		byAddress := make(map[common.Address]*curve.Pool, len(pools))
		for _, pool := range pools {
			byAddress[pool.Address] = pool
		}

		for _, route := range routes {
			name, fee := "", 0.0
			if pool, ok := byAddress[route.Pool]; ok {
				name, fee = pool.Name, pool.Fee()
			}

			if len(name) > 20 {
				name = name[:20]
			}

			// the registry does not convert a lending pool's underlying
			// balances, so printing one in the output coin's decimals would be
			// orders of magnitude out
			held, marker := quote.FormatRaw(route.OutBalance), ""
			if !route.OutBalanceExact {
				held, marker = "-", " (underlying)"
			}

			fmt.Printf("  %-44s %-20s %7.3f%% %5d %5d %26s%s\n",
				route.Pool.Hex(), name, fee*100, route.I, route.J, held, marker)
		}

		return nil
	},
}

// go run ./cmd/bbgo curve pool --session=ethereum --address=0xbEbc...
var curvePoolCmd = &cobra.Command{
	Use:          "pool",
	Short:        "Show one pool's coins, balances and parameters",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newCurveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		addressFlag, err := cmd.Flags().GetString("address")
		if err != nil {
			return err
		}

		if addressFlag == "" {
			return fmt.Errorf("--address is required")
		}

		if !common.IsHexAddress(addressFlag) {
			return fmt.Errorf("%q is not an address", addressFlag)
		}

		pool, err := client.Pool(ctx, common.HexToAddress(addressFlag))
		if err != nil {
			return err
		}

		name := pool.Name
		if name == "" {
			name = "(the registry has no name for this pool)"
		}

		fmt.Printf("CURVE POOL %s on %s\n", pool.Address.Hex(), client.ChainName())
		fmt.Printf("  NAME:      %s\n", name)
		fmt.Printf("  FEE:       %.4f%%\n", pool.Fee()*100)

		if pool.A != nil {
			fmt.Printf("  A:         %s\n", pool.A)
		}

		if pool.IsMeta {
			fmt.Printf("  BASE POOL: %s\n", pool.BasePool.Hex())
		}

		if pool.Index != curve.IndexUnknown {
			fmt.Printf("  INDEX:     %s\n", pool.Index)
		}

		printCurveCoins(ctx, session, "COINS", pool.Coins, pool.Balances)
		printCurveCoins(ctx, session, "UNDERLYING COINS", pool.UnderlyingCoins, pool.UnderlyingBalances)

		return nil
	},
}

func printCurveCoins(
	ctx context.Context, session *bbgo.ChainSession,
	title string, coins []common.Address, balances []*big.Int,
) {
	if len(coins) == 0 {
		return
	}

	fmt.Printf("\n  %s\n", title)
	fmt.Printf("    %5s %-44s %-10s %26s\n", "INDEX", "ADDRESS", "SYMBOL", "BALANCE")

	registry := session.TokenRegistry()

	for i, coin := range coins {
		symbol := ""
		formatted := ""

		if t, err := registry.Lookup(ctx, coin); err == nil {
			symbol = t.Symbol

			if i < len(balances) {
				formatted = t.FormatRaw(balances[i])
			}
		} else if i < len(balances) && balances[i] != nil {
			formatted = balances[i].String()
		}

		fmt.Printf("    %5d %-44s %-10s %26s\n", i, coin.Hex(), symbol, formatted)
	}
}

// go run ./cmd/bbgo curve quote --session=ethereum --from=USDC --to=USDT --amount=1000
var curveQuoteCmd = &cobra.Command{
	Use:          "quote",
	Short:        "Quote a swap across the pools that trade the pair",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newCurveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		from, to, amountIn, err := resolveCurveSwapArgs(ctx, cmd, session)
		if err != nil {
			return err
		}

		quotes, err := client.Quotes(ctx, from.Address, to.Address, amountIn)
		if err != nil {
			return err
		}

		fmt.Printf("QUOTE %s %s -> %s on %s\n",
			from.FormatRaw(amountIn), from.Symbol, to.Symbol, client.ChainName())
		fmt.Printf("  %-44s %26s %18s %-12s %s\n", "POOL", "OUTPUT", "PRICE", "KIND", "CALL")

		for _, quote := range quotes {
			price, _ := quote.Price(from.Decimals, to.Decimals).Float64()

			fmt.Printf("  %-44s %26s %18.8f %-12s %s(%d, %d)\n",
				quote.Route.Pool.Hex(),
				to.FormatRaw(quote.AmountOut),
				price,
				quote.Route.Index,
				quote.Route.Method(), quote.Route.I, quote.Route.J)
		}

		best := quotes[0]

		fmt.Printf("\n  BEST: %s %s via %s\n",
			to.FormatRaw(best.AmountOut), to.Symbol, best.Route.Pool.Hex())

		// a quote is the pool's own arithmetic, not a promise: a deprecated
		// pool keeps quoting a price it can no longer honour
		if len(quotes) > 1 {
			fmt.Printf("  a quote is what the pool computes, not what it can pay out;" +
				" the swap simulation is what proves it\n")
		}

		return nil
	},
}

// go run ./cmd/bbgo curve swap --session=ethereum --from=USDC --to=USDT --amount=100 --confirm
var curveSwapCmd = &cobra.Command{
	Use:          "swap",
	Short:        "Swap one coin for another through a Curve pool (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newCurveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		from, to, amountIn, err := resolveCurveSwapArgs(ctx, cmd, session)
		if err != nil {
			return err
		}

		slippage, err := cmd.Flags().GetInt("slippage")
		if err != nil {
			return err
		}

		poolFlag, err := cmd.Flags().GetString("pool")
		if err != nil {
			return err
		}

		confirm, err := cmd.Flags().GetBool("confirm")
		if err != nil {
			return err
		}

		if !client.CanWrite() {
			return fmt.Errorf(
				"chain session %s is read-only: set %s_PRIVATE_KEY to send transactions",
				session.Name, strings.ToUpper(session.Name))
		}

		var pool common.Address

		if poolFlag != "" {
			if !common.IsHexAddress(poolFlag) {
				return fmt.Errorf("%q is not an address", poolFlag)
			}

			pool = common.HexToAddress(poolFlag)
		}

		// the quote is a real eth_call against the pool, so this plan is a
		// genuine feasibility and price check, not a guess
		var quote *curve.Quote

		if pool == (common.Address{}) {
			quote, err = client.BestQuote(ctx, from.Address, to.Address, amountIn)
		} else {
			var route *curve.Route

			route, err = client.RouteForPool(ctx, pool, from.Address, to.Address)
			if err == nil {
				quote, err = client.Quote(ctx, route, amountIn)
			}
		}

		if err != nil {
			return err
		}

		minimumOut := quote.AmountOutMinimum(slippage)
		price, _ := quote.Price(from.Decimals, to.Decimals).Float64()

		fmt.Printf("SWAP %s %s -> %s on %s\n",
			from.FormatRaw(amountIn), from.Symbol, to.Symbol, client.ChainName())
		fmt.Printf("  ACCOUNT: %s\n", session.AddressHex())
		fmt.Printf("  POOL:    %s (%s)\n", quote.Route.Pool.Hex(), quote.Route.Index)
		fmt.Printf("  CALL:    %s(%d, %d)\n",
			quote.Route.Method(), quote.Route.I, quote.Route.J)
		fmt.Printf("  QUOTED:  %s %s (%.8f %s per %s)\n",
			to.FormatRaw(quote.AmountOut), to.Symbol, price, to.Symbol, from.Symbol)
		fmt.Printf("  FLOOR:   %s %s at %d bps of slippage\n",
			to.FormatRaw(minimumOut), to.Symbol, slippage)

		if !confirm {
			fmt.Printf("\nnot sent. pass --confirm to swap.\n")
			return nil
		}

		result, err := client.Swap(ctx, curve.SwapRequest{
			TokenIn:     from.Address,
			TokenOut:    to.Address,
			AmountIn:    amountIn,
			Pool:        quote.Route.Pool,
			SlippageBps: slippage,
		})

		if result != nil && result.Tx != nil {
			fmt.Printf("  HASH:     %s\n", result.Tx.Hash.Hex())
			fmt.Printf("  GAS COST: %s %s\n",
				token.FormatRaw(result.Tx.GasCost, session.EVMChain().NativeDecimals),
				session.EVMChain().NativeSymbol)
			fmt.Printf("  EXPLORER: %s\n", session.EVMChain().TxURL(result.Tx.Hash.Hex()))
		}

		if err != nil {
			return err
		}

		if result.AmountOut != nil {
			fmt.Printf("  FILLED:   %s %s (slippage %.4f%%)\n",
				to.FormatRaw(result.AmountOut), to.Symbol, result.Slippage()*100)
		}

		return nil
	},
}

// resolveCurveSwapArgs resolves the pair and the input amount of a quote or a
// swap command.
func resolveCurveSwapArgs(
	ctx context.Context, cmd *cobra.Command, session *bbgo.ChainSession,
) (from, to token.Token, amountIn *big.Int, err error) {
	from, to, err = resolvePair(ctx, cmd, session, "from", "to")
	if err != nil {
		return from, to, nil, err
	}

	amountFlag, err := cmd.Flags().GetString("amount")
	if err != nil {
		return from, to, nil, err
	}

	if amountFlag == "" {
		return from, to, nil, fmt.Errorf("--amount is required")
	}

	amountIn, err = token.ParseAmount(amountFlag, from.Decimals)

	return from, to, amountIn, err
}
