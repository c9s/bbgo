package types

import (
	"encoding/json"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
)

type BalanceType string

const (
	BalanceTypeAvailable BalanceType = "AVAILABLE"
	BalanceTypeLocked    BalanceType = "LOCKED"
	BalanceTypeBorrowed  BalanceType = "BORROWED"
	BalanceTypeInterest  BalanceType = "INTEREST"
	BalanceTypeNet       BalanceType = "NET"
	BalanceTypeTotal     BalanceType = "TOTAL"
	BalanceTypeDebt      BalanceType = "DEBT"
)

var ErrInvalidBalanceType = errors.New("invalid balance type")

func ParseBalanceType(s string) (b BalanceType, err error) {
	b = BalanceType(strings.ToUpper(s))
	switch b {
	case BalanceTypeAvailable, BalanceTypeLocked, BalanceTypeBorrowed, BalanceTypeInterest, BalanceTypeNet, BalanceTypeTotal, BalanceTypeDebt:
		return b, nil
	}
	return b, ErrInvalidBalanceType
}

func (b *BalanceType) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	t, err := ParseBalanceType(s)
	if err != nil {
		return err
	}

	*b = t

	return nil
}

func (b BalanceType) Map(balance Balance) fixedpoint.Value {
	switch b {
	case BalanceTypeAvailable:
		return balance.Available
	case BalanceTypeLocked:
		return balance.Locked
	case BalanceTypeBorrowed:
		return balance.Borrowed
	case BalanceTypeInterest:
		return balance.Interest
	case BalanceTypeNet:
		return balance.Net()
	case BalanceTypeTotal:
		return balance.Total()
	case BalanceTypeDebt:
		return balance.Debt()
	}
	return fixedpoint.Zero
}
