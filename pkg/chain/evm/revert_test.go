package evm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dataError mimics the RPC error that carries the revert return data.
type dataError struct {
	msg  string
	data any
}

func (e *dataError) Error() string  { return e.msg }
func (e *dataError) ErrorData() any { return e.data }
func (e *dataError) ErrorCode() int { return 3 }

func revertPayload(t *testing.T, reason string) string {
	t.Helper()

	stringType, err := abi.NewType("string", "", nil)
	require.NoError(t, err)

	encoded, err := abi.Arguments{{Type: stringType}}.Pack(reason)
	require.NoError(t, err)

	// the Error(string) selector
	return "0x08c379a0" + fmt.Sprintf("%x", encoded)
}

func TestRevertReasonFromReturnData(t *testing.T) {
	err := &dataError{msg: "execution reverted", data: revertPayload(t, "STF")}

	reason, ok := RevertReason(err)
	assert.True(t, ok)
	assert.Equal(t, "STF", reason)
}

func TestRevertReasonFromCustomError(t *testing.T) {
	err := &dataError{msg: "execution reverted", data: "0xdeadbeef"}

	reason, ok := RevertReason(err)
	assert.True(t, ok)
	assert.Equal(t, "custom error 0xdeadbeef", reason)
}

func TestRevertReasonFromMessage(t *testing.T) {
	reason, ok := RevertReason(errors.New("execution reverted: INSUFFICIENT_OUTPUT_AMOUNT"))
	assert.True(t, ok)
	assert.Equal(t, "INSUFFICIENT_OUTPUT_AMOUNT", reason)
}

func TestRevertReasonNonRevert(t *testing.T) {
	_, ok := RevertReason(errors.New("connection reset by peer"))
	assert.False(t, ok)

	_, ok = RevertReason(nil)
	assert.False(t, ok)
}

func TestWrapRevert(t *testing.T) {
	assert.NoError(t, WrapRevert("simulation", nil))

	err := WrapRevert("simulation", errors.New("execution reverted: STF"))
	assert.ErrorIs(t, err, ErrReverted)
	assert.ErrorContains(t, err, "simulation failed")
	assert.ErrorContains(t, err, "STF")

	// a bare revert without a reason still maps to ErrReverted
	err = WrapRevert("gas estimation", errors.New("execution reverted"))
	assert.ErrorIs(t, err, ErrReverted)

	// anything else is passed through
	cause := errors.New("connection reset by peer")
	err = WrapRevert("simulation", cause)
	assert.NotErrorIs(t, err, ErrReverted)
	assert.ErrorIs(t, err, cause)
}
