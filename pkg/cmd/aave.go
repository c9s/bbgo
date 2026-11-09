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
	v3 "github.com/c9s/bbgo/pkg/defi/aave/v3"
)

func init() {
	aaveCmd.PersistentFlags().String("session", "", "the chain session name")

	aaveMarketCmd.Flags().String("asset", "", "show only this asset")

	aaveAccountCmd.Flags().String("address", "", "the account to inspect, defaults to the session address")

	for _, cmd := range []*cobra.Command{aaveSupplyCmd, aaveWithdrawCmd, aaveBorrowCmd, aaveRepayCmd} {
		cmd.Flags().String("asset", "", "the asset symbol or address")
		cmd.Flags().String("amount", "", "the amount, as a decimal number")
		cmd.Flags().Bool("confirm", false, "actually send the transaction")
	}

	aaveWithdrawCmd.Flags().Bool("all", false, "withdraw the whole balance")
	aaveRepayCmd.Flags().Bool("all", false, "repay the whole debt")

	aaveCmd.AddCommand(aaveMarketCmd)
	aaveCmd.AddCommand(aaveAccountCmd)
	aaveCmd.AddCommand(aaveSupplyCmd)
	aaveCmd.AddCommand(aaveWithdrawCmd)
	aaveCmd.AddCommand(aaveBorrowCmd)
	aaveCmd.AddCommand(aaveRepayCmd)
	RootCmd.AddCommand(aaveCmd)
}

var aaveCmd = &cobra.Command{
	Use:          "aave",
	Short:        "Aave v3 lending market utilities",
	SilenceUsage: true,
}

// newAaveClient builds an Aave v3 client for a chain session. The sender is
// only passed when the session actually has one, so a read-only session does
// not end up with a typed-nil sender.
func newAaveClient(
	ctx context.Context, cmd *cobra.Command,
) (*bbgo.Environment, *bbgo.ChainSession, *v3.Client, error) {
	environ, session, err := selectChainSession(ctx, cmd)
	if err != nil {
		return nil, nil, nil, err
	}

	options := v3.Options{
		Tokens:    session.TokenRegistry(),
		Allowance: session.AllowanceConfig(),
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

// go run ./cmd/bbgo aave market --session=bsc
var aaveMarketCmd = &cobra.Command{
	Use:          "market",
	Short:        "Show the Aave v3 reserves and their rates",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newAaveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		assetFlag, err := cmd.Flags().GetString("asset")
		if err != nil {
			return err
		}

		var filter *common.Address
		if assetFlag != "" {
			t, err := session.TokenRegistry().Resolve(ctx, assetFlag)
			if err != nil {
				return err
			}

			filter = &t.Address
		}

		reserves, err := client.Reserves(ctx)
		if err != nil {
			return err
		}

		addresses := client.Addresses()

		fmt.Printf("AAVE V3 %q on %s, pool %s\n",
			addresses.MarketID, session.EVMChain().Name, addresses.Pool.Hex())
		fmt.Printf("  %-10s %18s %18s %10s %10s %12s\n",
			"ASSET", "SUPPLIED", "DEBT", "SUPPLY", "BORROW", "UTILIZATION")

		sort.Slice(reserves, func(i, j int) bool {
			return reserves[i].SupplyAPY() > reserves[j].SupplyAPY()
		})

		for _, reserve := range reserves {
			if filter != nil && reserve.Asset != *filter {
				continue
			}

			asset, err := session.TokenRegistry().Lookup(ctx, reserve.Asset)
			if err != nil {
				return err
			}

			fmt.Printf("  %-10s %18.4f %18.4f %9.3f%% %9.3f%% %11.1f%%\n",
				asset.Symbol,
				asset.ToFixedPointOrZero(reserve.TotalAToken),
				asset.ToFixedPointOrZero(reserve.TotalDebt()),
				reserve.SupplyAPY()*100,
				reserve.BorrowAPY()*100,
				utilization(reserve.TotalDebt(), reserve.TotalAToken)*100)
		}

		return nil
	},
}

func utilization(debt, supplied *big.Int) float64 {
	if supplied == nil || supplied.Sign() == 0 {
		return 0
	}

	ratio, _ := new(big.Float).Quo(new(big.Float).SetInt(debt), new(big.Float).SetInt(supplied)).Float64()

	return ratio
}

// go run ./cmd/bbgo aave account --session=bsc
var aaveAccountCmd = &cobra.Command{
	Use:          "account",
	Short:        "Show an Aave v3 account position and its health factor",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		environ, session, client, err := newAaveClient(ctx, cmd)
		if err != nil {
			return err
		}

		defer environ.CloseChainSessions()

		addressFlag, err := cmd.Flags().GetString("address")
		if err != nil {
			return err
		}

		user := session.EVMAddress()
		if addressFlag != "" {
			if !common.IsHexAddress(addressFlag) {
				return fmt.Errorf("invalid address %q", addressFlag)
			}

			user = common.HexToAddress(addressFlag)
		}

		if user == (common.Address{}) {
			return fmt.Errorf("no address to inspect: pass --address, or configure one on the chain session")
		}

		account, err := client.AccountData(ctx, user)
		if err != nil {
			return err
		}

		fmt.Printf("AAVE V3 ACCOUNT %s on %s\n", user.Hex(), session.EVMChain().Name)
		fmt.Printf("  COLLATERAL:   %.2f\n", account.TotalCollateral())
		fmt.Printf("  DEBT:         %.2f\n", account.TotalDebt())
		fmt.Printf("  AVAILABLE:    %.2f\n", account.AvailableBorrows())
		fmt.Printf("  LTV:          %.2f%%\n", v3.BpsToFloat(account.LTVBps)*100)
		fmt.Printf("  LIQ. THRESH.: %.2f%%\n", v3.BpsToFloat(account.LiquidationThresholdBps)*100)

		if account.HasDebt() {
			fmt.Printf("  HEALTH:       %.4f\n", account.HealthFactorFloat())

			if account.IsLiquidatable() {
				fmt.Printf("  WARNING:      the health factor is below 1, this position can be liquidated\n")
			}
		} else {
			fmt.Printf("  HEALTH:       no debt\n")
		}

		positions, err := client.UserReserves(ctx, user)
		if err != nil {
			return err
		}

		if len(positions) == 0 {
			return nil
		}

		fmt.Printf("  %-10s %24s %24s %12s\n", "ASSET", "SUPPLIED", "DEBT", "COLLATERAL")

		for _, position := range positions {
			asset, err := session.TokenRegistry().Lookup(ctx, position.Asset)
			if err != nil {
				return err
			}

			fmt.Printf("  %-10s %24s %24s %12v\n",
				asset.Symbol,
				asset.FormatRaw(position.ATokenBalance),
				asset.FormatRaw(position.TotalDebt()),
				position.UsageAsCollateralEnabled)
		}

		return nil
	},
}

// aaveAction resolves the asset and the amount shared by the write commands.
func aaveAction(
	ctx context.Context, cmd *cobra.Command, session *bbgo.ChainSession, allowAll bool,
) (t token.Token, amount *big.Int, confirm bool, err error) {
	assetFlag, err := cmd.Flags().GetString("asset")
	if err != nil {
		return t, nil, false, err
	}

	amountFlag, err := cmd.Flags().GetString("amount")
	if err != nil {
		return t, nil, false, err
	}

	confirm, err = cmd.Flags().GetBool("confirm")
	if err != nil {
		return t, nil, false, err
	}

	all := false
	if allowAll {
		all, err = cmd.Flags().GetBool("all")
		if err != nil {
			return t, nil, false, err
		}
	}

	if all == (amountFlag != "") {
		if allowAll {
			return t, nil, false, fmt.Errorf("pass either --amount or --all")
		}

		return t, nil, false, fmt.Errorf("--amount is required")
	}

	t, err = session.TokenRegistry().Resolve(ctx, assetFlag)
	if err != nil {
		return t, nil, false, err
	}

	if all {
		return t, v3.MaxAmount, confirm, nil
	}

	amount, err = token.ParseAmount(amountFlag, t.Decimals)
	if err != nil {
		return t, nil, false, err
	}

	return t, amount, confirm, nil
}

// runAaveAction prints what would happen and only sends with --confirm.
//
// Unlike bbgo chain send, this cannot dry-run through a simulation: supplying
// and repaying revert until the pool is approved, so simulating an unapproved
// first supply would report a failure that the real path does not have.
func runAaveAction(
	cmd *cobra.Command,
	verb string,
	allowAll bool,
	action func(ctx context.Context, client *v3.Client, asset common.Address, amount *big.Int) (string, error),
) error {
	ctx := context.Background()

	environ, session, client, err := newAaveClient(ctx, cmd)
	if err != nil {
		return err
	}

	defer environ.CloseChainSessions()

	t, amount, confirm, err := aaveAction(ctx, cmd, session, allowAll)
	if err != nil {
		return err
	}

	if !client.CanWrite() {
		return fmt.Errorf(
			"chain session %s is read-only: set %s_PRIVATE_KEY to send transactions",
			session.Name, strings.ToUpper(session.Name))
	}

	target := t.FormatRaw(amount)
	if amount.Cmp(v3.MaxAmount) == 0 {
		target = "the whole position"
	}

	fmt.Printf("AAVE %s %s %s on %s\n",
		strings.ToUpper(verb), target, t.Symbol, session.EVMChain().Name)
	fmt.Printf("  POOL:    %s\n", client.PoolAddress())
	fmt.Printf("  ACCOUNT: %s\n", session.AddressHex())

	if account, err := client.AccountData(ctx, session.EVMAddress()); err == nil {
		fmt.Printf("  BEFORE:  %s\n", account)
	}

	if !confirm {
		fmt.Printf("\nnot sent. pass --confirm to send, it is not simulated first because\n" +
			"supplying and repaying revert until the pool is approved.\n")

		return nil
	}

	summary, err := action(ctx, client, t.Address, amount)
	if err != nil {
		return err
	}

	fmt.Printf("  RESULT:  %s\n", summary)

	if account, err := client.AccountData(ctx, session.EVMAddress()); err == nil {
		fmt.Printf("  AFTER:   %s\n", account)
	}

	return nil
}

var aaveSupplyCmd = &cobra.Command{
	Use:          "supply",
	Short:        "Supply an asset to Aave v3 (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAaveAction(cmd, "supply", false,
			func(ctx context.Context, client *v3.Client, asset common.Address, amount *big.Int) (string, error) {
				result, err := client.Supply(ctx, asset, amount)
				if err != nil {
					return "", err
				}

				return result.String(), nil
			})
	},
}

var aaveWithdrawCmd = &cobra.Command{
	Use:          "withdraw",
	Short:        "Withdraw an asset from Aave v3 (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAaveAction(cmd, "withdraw", true,
			func(ctx context.Context, client *v3.Client, asset common.Address, amount *big.Int) (string, error) {
				result, err := client.Withdraw(ctx, asset, amount)
				if err != nil {
					return "", err
				}

				return result.String(), nil
			})
	},
}

var aaveBorrowCmd = &cobra.Command{
	Use:          "borrow",
	Short:        "Borrow an asset from Aave v3 at the variable rate (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAaveAction(cmd, "borrow", false,
			func(ctx context.Context, client *v3.Client, asset common.Address, amount *big.Int) (string, error) {
				result, err := client.Borrow(ctx, asset, amount)
				if err != nil {
					return "", err
				}

				return result.String(), nil
			})
	},
}

var aaveRepayCmd = &cobra.Command{
	Use:          "repay",
	Short:        "Repay an Aave v3 loan (sends only with --confirm)",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAaveAction(cmd, "repay", true,
			func(ctx context.Context, client *v3.Client, asset common.Address, amount *big.Int) (string, error) {
				result, err := client.Repay(ctx, asset, amount)
				if err != nil {
					return "", err
				}

				return result.String(), nil
			})
	},
}
