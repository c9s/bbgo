package bybit

import (
	"context"
	"fmt"
	"strconv"
)

func (e *Exchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if e.IsFutures {
		_, err := e.client.NewSetPositionLevelRequest().
			Symbol(symbol).
			BuyLeverage(strconv.Itoa(leverage)).
			SellLeverage(strconv.Itoa(leverage)).
			Do(ctx)
		return err
	}

	return fmt.Errorf("not supported set leverage")
}
