package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
	v3 "github.com/c9s/bbgo/pkg/defi/uniswap/v3"
)

func init() {
	uniswapCmd.PersistentFlags().String("session", "", "the chain session name")
	uniswapCmd.PersistentFlags().String("dex", "", "the deployment to use, for example uniswap or pancakeswap")

	uniswapPoolsCmd.Flags().String("base", "", "the base token symbol or address")
	uniswapPoolsCmd.Flags().String("quote", "", "the quote token symbol or address")

	uniswapQuoteCmd.Flags().String("from", "", "the input token symbol or address")
	uniswapQuoteCmd.Flags().String("to", "", "the output token symbol or address")
	uniswapQuoteCmd.Flags().String("amount", "", "the input amount, as a decimal number")

	uniswapSwapCmd.Flags().String("from", "", "the input token symbol or address")
	uniswapSwapCmd.Flags().String("to", "", "the output token symbol or address")
	uniswapSwapCmd.Flags().String("amount", "", "the input amount, as a decimal number")
	uniswapSwapCmd.Flags().Int("slippage", 50, "the slippage tolerance in basis points")
	uniswapSwapCmd.Flags().Uint32("fee", 0, "the fee tier to use; the best tier is chosen when omitted")
	uniswapSwapCmd.Flags().Bool("confirm", false, "actually send the swap")

	uniswapCmd.AddCommand(uniswapPoolsCmd)
	uniswapCmd.AddCommand(uniswapQuoteCmd)
	uniswapCmd.AddCommand(uniswapSwapCmd)
	RootCmd.AddCommand(uniswapCmd)
}

var uniswapCmd = &cobra.Command{
	Use:          "uniswap",
	Short:        "Uniswap v3 utilities, including the v3 forks such as PancakeSwap",
	SilenceUsage: true,
}

func newUniswapClient(
	ctx context.Context, cmd *cobra.Command,
) (*bbgo.Environment, *bbgo.ChainSession, *v3.Client, error) {
	environ, session, err := selectChainSession(ctx, cmd)
	if err != nil {
		return nil, nil, nil, err
	}

	dex, err := cmd.Flags().GetString("dex")
	if err != nil {
		environ.CloseChainSessions()
		return nil, nil, nil, err
	}

	options := v3.Options{
		Deployment: dex,
		Tokens:     session.TokenRegistry(),
		Allowance:  session.AllowanceConfig(),
	}

	if sender := session.Sender(); sender != nil {
		options.Sender = sender
	}

	client, err := v3.NewClient(ctx, session.EVMChain(), session.Client(), options)
	if err != nil {
		environ.CloseChainSessions()
		return nil, nil, nil, err
	}

	return environ, session, client, nil
}

// resolvePair resolves two token flags into their metadata.
func resolvePair(
	ctx context.Context, cmd *cobra.Command, session *bbgo.ChainSession, fromFlag, toFlag string,
) (from, to token.Token, err error) {
	fromRef, err := cmd.Flags().GetString(fromFlag)
	if err != nil {
		return from, to, err
	}

	toRef, err := cmd.Flags().GetString(toFlag)
	if err != nil {
		return from, to, err
	}

	if fromRef == "" || toRef == "" {
		return from, to, fmt.Errorf("--%s and --%s are required", fromFlag, toFlag)
	}

	registry := session.TokenRegistry()

	from, err = registry.Resolve(ctx, fromRef)
	if err != nil {
		return from, to, err
	}

	to, err = registry.Resolve(ctx, toRef)

	return from, to, err
}

// go run ./cmd/bbgo uniswap pools --session=bsc --base=WBNB --quote=USDT
var uniswapPoolsCmd = &cobra.Command{
	Use:          "pools",
	Short:        "List the v3 pools of a pair with their liquidity and spot price",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newUniswapClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		base, quote, err := resolvePair(ctx, cmd, session, "base", "quote")
		if err != nil {
			return err
		}

		pools, err := client.FindPools(ctx, base.Address, quote.Address)
		if err != nil {
			return err
		}

		if len(pools) == 0 {
			return fmt.Errorf("no %s/%s pool exists on %s", base.Symbol, quote.Symbol, client.Deployment().Name)
		}

		fmt.Printf("%s/%s on %s (%s)\n",
			base.Symbol, quote.Symbol, client.Deployment().Name, session.EVMChain().Name)
		fmt.Printf("  %-44s %8s %26s %18s\n", "POOL", "FEE", "LIQUIDITY", "SPOT")

		for _, pool := range pools {
			fmt.Printf("  %-44s %7.2f%% %26s %18.8f\n",
				pool.Address.Hex(),
				v3.FeeTierFraction(pool.Fee)*100,
				pool.Liquidity,
				poolSpot(pool, base, quote))
		}

		return nil
	},
}

// poolSpot orients the pool price as quote per base, whichever way the pool
// sorted the pair.
func poolSpot(pool *v3.Pool, base, quote token.Token) float64 {
	decimals0, decimals1 := base.Decimals, quote.Decimals
	inverted := pool.Token0 != base.Address

	if inverted {
		decimals0, decimals1 = quote.Decimals, base.Decimals
	}

	price, _ := pool.Price(decimals0, decimals1).Float64()

	if inverted && price != 0 {
		return 1 / price
	}

	return price
}

// go run ./cmd/bbgo uniswap quote --session=bsc --from=WBNB --to=USDT --amount=1
var uniswapQuoteCmd = &cobra.Command{
	Use:          "quote",
	Short:        "Quote a swap across every fee tier",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newUniswapClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		from, to, err := resolvePair(ctx, cmd, session, "from", "to")
		if err != nil {
			return err
		}

		amountFlag, err := cmd.Flags().GetString("amount")
		if err != nil {
			return err
		}

		amountIn, err := token.ParseAmount(amountFlag, from.Decimals)
		if err != nil {
			return err
		}

		fmt.Printf("QUOTE %s %s -> %s on %s (%s)\n",
			from.FormatRaw(amountIn), from.Symbol, to.Symbol,
			client.Deployment().Name, session.EVMChain().Name)
		fmt.Printf("  %8s %26s %18s %10s\n", "FEE", "OUTPUT", "PRICE", "TICKS")

		var best *v3.Quote

		for _, fee := range client.Deployment().FeeTiers {
			quote, err := client.Quote(ctx, from.Address, to.Address, fee, amountIn)
			if err != nil {
				fmt.Printf("  %7.2f%% %26s\n", v3.FeeTierFraction(fee)*100, "no pool")
				continue
			}

			price, _ := quote.Price(from.Decimals, to.Decimals).Float64()

			fmt.Printf("  %7.2f%% %26s %18.8f %10d\n",
				v3.FeeTierFraction(fee)*100,
				to.FormatRaw(quote.AmountOut),
				price,
				quote.InitializedTicksCrossed)

			if best == nil || quote.AmountOut.Cmp(best.AmountOut) > 0 {
				best = quote
			}
		}

		if best == nil {
			return fmt.Errorf("no pool quotes %s/%s on %s", from.Symbol, to.Symbol, client.Deployment().Name)
		}

		fmt.Printf("\n  BEST: %s %s at the %.2f%% tier\n",
			to.FormatRaw(best.AmountOut), to.Symbol, v3.FeeTierFraction(best.Fee)*100)

		return nil
	},
}

// go run ./cmd/bbgo uniswap swap --session=bsc --from=USDT --to=WBNB --amount=10 --confirm
var uniswapSwapCmd = &cobra.Command{
	Use:          "swap",
	Short:        "Swap one token for another (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newUniswapClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		from, to, err := resolvePair(ctx, cmd, session, "from", "to")
		if err != nil {
			return err
		}

		amountFlag, err := cmd.Flags().GetString("amount")
		if err != nil {
			return err
		}

		slippage, err := cmd.Flags().GetInt("slippage")
		if err != nil {
			return err
		}

		fee, err := cmd.Flags().GetUint32("fee")
		if err != nil {
			return err
		}

		confirm, err := cmd.Flags().GetBool("confirm")
		if err != nil {
			return err
		}

		amountIn, err := token.ParseAmount(amountFlag, from.Decimals)
		if err != nil {
			return err
		}

		if !client.CanWrite() {
			return fmt.Errorf(
				"chain session %s is read-only: set %s_PRIVATE_KEY to send transactions",
				session.Name, strings.ToUpper(session.Name))
		}

		// the quote is a real eth_call against the quoter, so this plan is a
		// genuine feasibility and price check, not a guess
		var quote *v3.Quote
		if fee == 0 {
			quote, err = client.BestQuote(ctx, from.Address, to.Address, amountIn)
		} else {
			quote, err = client.Quote(ctx, from.Address, to.Address, fee, amountIn)
		}

		if err != nil {
			return err
		}

		minimumOut := quote.AmountOutMinimum(slippage)
		price, _ := quote.Price(from.Decimals, to.Decimals).Float64()

		fmt.Printf("SWAP %s %s -> %s on %s (%s)\n",
			from.FormatRaw(amountIn), from.Symbol, to.Symbol,
			client.Deployment().Name, session.EVMChain().Name)
		fmt.Printf("  ACCOUNT:  %s\n", session.AddressHex())
		fmt.Printf("  ROUTER:   %s\n", client.RouterAddress().Hex())
		fmt.Printf("  FEE TIER: %.2f%%\n", v3.FeeTierFraction(quote.Fee)*100)
		fmt.Printf("  QUOTED:   %s %s (%.8f %s per %s)\n",
			to.FormatRaw(quote.AmountOut), to.Symbol, price, to.Symbol, from.Symbol)
		fmt.Printf("  FLOOR:    %s %s at %d bps of slippage\n",
			to.FormatRaw(minimumOut), to.Symbol, slippage)

		if !confirm {
			fmt.Printf("\nnot sent. pass --confirm to swap.\n")
			return nil
		}

		result, err := client.Swap(ctx, v3.SwapRequest{
			TokenIn:     from.Address,
			TokenOut:    to.Address,
			Fee:         quote.Fee,
			AmountIn:    amountIn,
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
