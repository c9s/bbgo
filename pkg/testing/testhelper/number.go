package testhelper

import "github.com/c9s/bbgo/pkg/fixedpoint"

func Number(a interface{}) fixedpoint.Value {
	switch v := a.(type) {
	case string:
		return fixedpoint.MustNewFromString(v)
	case int:
		return fixedpoint.NewFromInt(int64(v))
	case int64:
		return fixedpoint.NewFromInt(int64(v))
	case float64:
		return fixedpoint.NewFromFloat(v)
	}

	return fixedpoint.Zero
}
