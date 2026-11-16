package token

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// MaxUint256 is the allowance value of an unlimited approval.
var MaxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// AllowancePolicy decides how much is approved when a top-up is needed.
type AllowancePolicy string

const (
	// AllowanceExact approves only what the current operation needs. It is the
	// default: a compromised or buggy spender can take no more than the amount
	// of one operation.
	AllowanceExact AllowancePolicy = "exact"

	// AllowanceUnlimited approves MaxUint256 once, which saves the gas of every
	// later approval at the cost of an unbounded exposure to the spender.
	AllowanceUnlimited AllowancePolicy = "unlimited"
)

// AllowanceConfig configures the approval behaviour.
type AllowanceConfig struct {
	Policy AllowancePolicy `json:"policy,omitempty" yaml:"policy,omitempty"`

	// ResetBeforeApprove first approves zero when the current allowance is
	// non-zero. Some tokens refuse to change a non-zero allowance directly.
	ResetBeforeApprove bool `json:"resetBeforeApprove,omitempty" yaml:"resetBeforeApprove,omitempty"`
}

// ResolvedPolicy returns the effective policy, defaulting to AllowanceExact.
func (c AllowanceConfig) ResolvedPolicy() AllowancePolicy {
	switch AllowancePolicy(strings.ToLower(string(c.Policy))) {
	case AllowanceUnlimited:
		return AllowanceUnlimited
	default:
		return AllowanceExact
	}
}

// TxSender is the transaction-sending capability that approving needs.
type TxSender interface {
	Address() common.Address
	SendAndWait(ctx context.Context, req evm.SendRequest) (*evm.TxResult, error)
}

// EnsureAllowance approves spender for amount when the current allowance is
// below it, and returns the approval result. A nil result with a nil error
// means the existing allowance was already sufficient and nothing was sent.
func EnsureAllowance(
	ctx context.Context,
	caller bind.ContractCaller,
	sender TxSender,
	t Token,
	spender common.Address,
	amount *big.Int,
	config AllowanceConfig,
) (*evm.TxResult, error) {
	if amount == nil || amount.Sign() <= 0 {
		return nil, fmt.Errorf("the allowance amount must be positive")
	}

	owner := sender.Address()

	current, err := Allowance(ctx, caller, t.Address, owner, spender)
	if err != nil {
		return nil, fmt.Errorf("failed to read the %s allowance of %s for %s: %w",
			t.Symbol, owner.Hex(), spender.Hex(), err)
	}

	if current.Cmp(amount) >= 0 {
		return nil, nil
	}

	logger := log.WithFields(log.Fields{
		"token":   t.Symbol,
		"spender": spender.Hex(),
		"owner":   owner.Hex(),
	})

	if config.ResetBeforeApprove && current.Sign() > 0 {
		logger.Infof("resetting the allowance to zero before approving")

		if _, err := approve(ctx, sender, t, spender, new(big.Int)); err != nil {
			return nil, err
		}
	}

	target := amount
	if config.ResolvedPolicy() == AllowanceUnlimited {
		target = MaxUint256
	}

	logger.Infof("approving %s (current allowance %s)", t.FormatRaw(target), t.FormatRaw(current))

	return approve(ctx, sender, t, spender, target)
}

// Approve sets the allowance of spender to exactly amount.
func Approve(
	ctx context.Context, sender TxSender, t Token, spender common.Address, amount *big.Int,
) (*evm.TxResult, error) {
	return approve(ctx, sender, t, spender, amount)
}

func approve(
	ctx context.Context, sender TxSender, t Token, spender common.Address, amount *big.Int,
) (*evm.TxResult, error) {
	data, err := ApproveCallData(spender, amount)
	if err != nil {
		return nil, err
	}

	address := t.Address

	return sender.SendAndWait(ctx, evm.SendRequest{
		To:    &address,
		Data:  data,
		Label: "approve " + t.Symbol,
	})
}

// TransferCallData encodes transfer(recipient, amount).
func TransferCallData(recipient common.Address, amount *big.Int) ([]byte, error) {
	return erc20TransferABI.Pack("transfer", recipient, amount)
}

// Transfer sends an ERC20 transfer and waits for it.
func Transfer(
	ctx context.Context, sender TxSender, t Token, recipient common.Address, amount *big.Int,
) (*evm.TxResult, error) {
	data, err := TransferCallData(recipient, amount)
	if err != nil {
		return nil, err
	}

	address := t.Address

	return sender.SendAndWait(ctx, evm.SendRequest{
		To:    &address,
		Data:  data,
		Label: "transfer " + t.Symbol,
	})
}
