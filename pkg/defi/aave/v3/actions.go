package v3

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// repayBufferBps is added to the allowance of a full repayment, because the
// debt keeps accruing between the approval and the repayment.
const repayBufferBps = 50

func (c *Client) send(ctx context.Context, method, label string, args ...any) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	data, err := poolABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", method, err)
	}

	pool := c.addresses.Pool

	return sender.SendAndWait(ctx, evm.SendRequest{To: &pool, Data: data, Label: label})
}

// ensureAllowance approves the pool to move amount of asset, following the
// session's approval policy.
func (c *Client) ensureAllowance(
	ctx context.Context, asset common.Address, amount *big.Int,
) error {
	sender, err := c.requireSender()
	if err != nil {
		return err
	}

	t, err := c.Token(ctx, asset)
	if err != nil {
		return err
	}

	result, err := token.EnsureAllowance(ctx, c.caller, sender, t, c.addresses.Pool, amount, c.allowance)
	if err != nil {
		return err
	}

	if result != nil {
		log.Infof("aave v3: approved the pool to spend %s %s, %s", t.FormatRaw(amount), t.Symbol, result)
	}

	return nil
}

// Supply deposits amount of asset as collateral, approving the pool first.
func (c *Client) Supply(ctx context.Context, asset common.Address, amount *big.Int) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("the supply amount must be positive")
	}

	if err := c.ensureAllowance(ctx, asset, amount); err != nil {
		return nil, err
	}

	return c.send(ctx, "supply", "aave supply",
		asset, amount, sender.Address(), uint16(ReferralCode))
}

// Withdraw redeems amount of asset. Pass MaxAmount to withdraw everything.
func (c *Client) Withdraw(ctx context.Context, asset common.Address, amount *big.Int) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("the withdraw amount must be positive, or MaxAmount for everything")
	}

	return c.send(ctx, "withdraw", "aave withdraw", asset, amount, sender.Address())
}

// Borrow takes a variable rate loan of amount of asset. The stable rate mode
// no longer exists in Aave v3.
func (c *Client) Borrow(ctx context.Context, asset common.Address, amount *big.Int) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("the borrow amount must be positive")
	}

	return c.send(ctx, "borrow", "aave borrow",
		asset, amount, big.NewInt(InterestRateModeVariable), uint16(ReferralCode), sender.Address())
}

// Repay pays back amount of a variable rate loan, approving the pool first.
// Passing MaxAmount is the same as calling RepayAll.
func (c *Client) Repay(ctx context.Context, asset common.Address, amount *big.Int) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("the repay amount must be positive, or MaxAmount for the whole debt")
	}

	if amount.Cmp(MaxAmount) == 0 {
		return c.RepayAll(ctx, asset)
	}

	if err := c.ensureAllowance(ctx, asset, amount); err != nil {
		return nil, err
	}

	return c.send(ctx, "repay", "aave repay",
		asset, amount, big.NewInt(InterestRateModeVariable), sender.Address())
}

// RepayAll pays back the whole variable rate debt of asset.
//
// The allowance is sized from the current debt plus a small buffer, because the
// debt keeps accruing between the approval and the repayment, and the repayment
// itself is sent with MaxAmount so the pool takes exactly what is owed.
func (c *Client) RepayAll(ctx context.Context, asset common.Address) (*evm.TxResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	position, err := c.UserReserve(ctx, asset, sender.Address())
	if err != nil {
		return nil, err
	}

	debt := position.TotalDebt()
	if debt.Sign() == 0 {
		return nil, fmt.Errorf("there is no debt to repay for %s", asset.Hex())
	}

	buffered := new(big.Int).Mul(debt, big.NewInt(10000+repayBufferBps))
	buffered.Div(buffered, big.NewInt(10000))

	if err := c.ensureAllowance(ctx, asset, buffered); err != nil {
		return nil, err
	}

	return c.send(ctx, "repay", "aave repay all",
		asset, MaxAmount, big.NewInt(InterestRateModeVariable), sender.Address())
}

// SetUseAsCollateral enables or disables an asset as collateral. Disabling an
// asset that backs a loan reverts.
func (c *Client) SetUseAsCollateral(
	ctx context.Context, asset common.Address, useAsCollateral bool,
) (*evm.TxResult, error) {
	return c.send(ctx, "setUserUseReserveAsCollateral", "aave set collateral", asset, useAsCollateral)
}
