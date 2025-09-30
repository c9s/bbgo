package types

import (
	"errors"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestRecoverOrderError(t *testing.T) {
	recoverOrderErr := &RecoverOrderError{
		Exchange: ExchangeMax,
		SubmitOrder: SubmitOrder{
			Price:    fixedpoint.NewFromFloat(200.0),
			Quantity: fixedpoint.NewFromFloat(10.0),
		},
		OriginalErr: errors.New("original error"),
		RecoverErr:  errors.New("recover error"),
		ResponseErr: nil,
	}

	err := error(recoverOrderErr)

	var toErr *RecoverOrderError
	if errors.As(err, &toErr) {
		t.Logf("as recover order error: %+v", toErr)
	} else {
		t.Errorf("cannot cast to recover order error")
	}

}
