package evm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// ErrReverted is the sentinel for a call or transaction that the contract
// rejected.
var ErrReverted = errors.New("execution reverted")

// RevertReason extracts the human readable revert reason out of an RPC error.
// Nodes report it either as return data attached to the error, which decodes
// through the standard Error(string) selector, or as part of the message text.
func RevertReason(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	var dataErr rpc.DataError
	if errors.As(err, &dataErr) {
		if raw, ok := dataErr.ErrorData().(string); ok && raw != "" {
			if data, decodeErr := hexutil.Decode(raw); decodeErr == nil && len(data) > 0 {
				if reason, unpackErr := abi.UnpackRevert(data); unpackErr == nil {
					return reason, true
				}

				// a custom error: the selector is all the information there is
				return "custom error " + hexutil.Encode(data[:min(4, len(data))]), true
			}

			return raw, true
		}
	}

	const prefix = "execution reverted:"
	if msg := err.Error(); strings.Contains(strings.ToLower(msg), prefix) {
		index := strings.Index(strings.ToLower(msg), prefix)
		if reason := strings.TrimSpace(msg[index+len(prefix):]); reason != "" {
			return reason, true
		}
	}

	return "", false
}

// WrapRevert turns an RPC error into an error wrapping ErrReverted when it is a
// revert, and returns it unchanged otherwise. The context describes what was
// being attempted, for example "simulate" or "estimate gas".
func WrapRevert(context string, err error) error {
	if err == nil {
		return nil
	}

	if reason, ok := RevertReason(err); ok {
		return fmt.Errorf("%s failed, %w: %s", context, ErrReverted, reason)
	}

	if strings.Contains(strings.ToLower(err.Error()), "execution reverted") {
		return fmt.Errorf("%s failed, %w", context, ErrReverted)
	}

	return fmt.Errorf("%s failed: %w", context, err)
}
