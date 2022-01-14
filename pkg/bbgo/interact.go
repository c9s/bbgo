package bbgo

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
)

type PositionCloser interface {
	ClosePosition(ctx context.Context, percentage float64) error
}

type PositionReader interface {
	CurrentPosition() *types.Position
}

type closePositionContext struct {
	signature  string
	closer     PositionCloser
	percentage float64
}

type CoreInteraction struct {
	environment *Environment
	trader      *Trader

	exchangeStrategies   map[string]SingleExchangeStrategy
	closePositionContext closePositionContext
}

func NewCoreInteraction(environment *Environment, trader *Trader) *CoreInteraction {
	return &CoreInteraction{
		environment:        environment,
		trader:             trader,
		exchangeStrategies: make(map[string]SingleExchangeStrategy),
	}
}

func (it *CoreInteraction) Commands(i *interact.Interact) {
	i.PrivateCommand("/position", "show the current position of a strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		found := false
		for signature, strategy := range it.exchangeStrategies {
			if _, ok := strategy.(PositionReader); ok {
				reply.AddButton(signature)
				found = true
			}
		}

		if found {
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports PositionReader")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		reader, implemented := strategy.(PositionReader)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support position close", signature))
			return fmt.Errorf("strategy %s does not implement PositionCloser interface", signature)
		}

		position := reader.CurrentPosition()
		if position != nil {
			reply.Send("Your current position:")
			reply.Send(position.PlainText())

			if position.Base == 0 {
				reply.Message(fmt.Sprintf("Strategy %q has no opened position", signature))
				return fmt.Errorf("strategy %T has no opened position", strategy)
			}
		}

		reply.RemoveKeyboard()
		return nil
	})

	i.PrivateCommand("/closeposition", "close the position of a strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		found := false
		for signature, strategy := range it.exchangeStrategies {
			if _, ok := strategy.(PositionCloser); ok {
				reply.AddButton(signature)
				found = true
			}
		}

		if found {
			reply.Message("Please choose your position from the current running strategies")
		} else {
			reply.Message("No any strategy supports PositionCloser")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		closer, implemented := strategy.(PositionCloser)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support position close", signature))
			return fmt.Errorf("strategy %s does not implement PositionCloser interface", signature)
		}

		it.closePositionContext.closer = closer
		it.closePositionContext.signature = signature

		if reader, implemented := strategy.(PositionReader); implemented {
			position := reader.CurrentPosition()
			if position != nil {
				reply.Send("Your current position:")
				reply.Send(position.PlainText())

				if position.Base == 0 {
					reply.Message("No opened position")
					reply.RemoveKeyboard()
					return fmt.Errorf("no opened position")
				}
			}
		}

		reply.Message("Choose or enter the percentage to close")
		for _, symbol := range []string{"5%", "25%", "50%", "80%", "100%"} {
			reply.AddButton(symbol)
		}

		return nil
	}).Next(func(percentageStr string, reply interact.Reply) error {
		percentage, err := parseFloatPercent(percentageStr, 64)
		if err != nil {
			reply.Message(fmt.Sprintf("%q is not a valid percentage string", percentageStr))
			return err
		}

		reply.RemoveKeyboard()

		err = it.closePositionContext.closer.ClosePosition(context.Background(), percentage)
		if err != nil {
			reply.Message(fmt.Sprintf("Failed to close the position, %s", err.Error()))
			return err
		}

		return nil
	})
}

func (it *CoreInteraction) Initialize() error {
	// re-map exchange strategies into the signature-object map
	for sessionID, strategies := range it.trader.exchangeStrategies {
		for _, strategy := range strategies {
			signature, err := getStrategySignature(strategy)
			if err != nil {
				return err
			}

			key := sessionID + "." + signature
			it.exchangeStrategies[key] = strategy
		}
	}
	return nil
}

func getStrategySignature(strategy SingleExchangeStrategy) (string, error) {
	rv := reflect.ValueOf(strategy).Elem()
	if rv.Kind() != reflect.Struct {
		return "", fmt.Errorf("strategy %T instance is not a struct", strategy)
	}

	var signature = path.Base(rv.Type().PkgPath())

	var id = strategy.ID()

	if !strings.EqualFold(id, signature) {
		signature += "." + strings.ToLower(id)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		if field.Kind() == reflect.String {
			str := field.String()
			if len(str) > 0 {
				signature += "." + field.String()
			}
		}
	}

	return signature, nil
}

func parseFloatPercent(s string, bitSize int) (f float64, err error) {
	i := strings.Index(s, "%")
	if i < 0 {
		return strconv.ParseFloat(s, bitSize)
	}

	f, err = strconv.ParseFloat(s[:i], bitSize)
	if err != nil {
		return 0, err
	}
	return f / 100.0, nil
}
