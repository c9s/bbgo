package bbgo

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
)

type PositionCloser interface {
	ClosePosition(ctx context.Context, percentage fixedpoint.Value) error
}

type PositionReader interface {
	CurrentPosition() *types.Position
}

type StrategyStatusProvider interface {
	GetStatus() types.StrategyStatus
}

type Suspender interface {
	Suspend(ctx context.Context) error
	Resume(ctx context.Context) error
}

type StrategyController interface {
	StrategyStatusProvider
	Suspender
}

type EmergencyStopper interface {
	EmergencyStop(ctx context.Context) error
}

type closePositionContext struct {
	signature  string
	closer     PositionCloser
	percentage fixedpoint.Value
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

func (it *CoreInteraction) AddSupportedStrategyButtons(checkInterface interface{}) ([][3]string, bool) {
	found := false
	var buttonsForm [][3]string
	rt := reflect.TypeOf(checkInterface).Elem()
	for signature, strategy := range it.exchangeStrategies {
		if ok := reflect.TypeOf(strategy).Implements(rt); ok {
			buttonsForm = append(buttonsForm, [3]string{signature, "strategy", signature})
			found = true
		}
	}

	return buttonsForm, found
}

func (it *CoreInteraction) Commands(i *interact.Interact) {
	i.PrivateCommand("/sessions", "List Exchange Sessions", func(reply interact.Reply) error {
		switch r := reply.(type) {
		case *interact.SlackReply:
			// call slack specific api to build the reply object
			_ = r
		}

		message := "Your connected sessions:\n"
		for name, session := range it.environment.Sessions() {
			message += "- " + name + " (" + session.ExchangeName.String() + ")\n"
		}

		reply.Message(message)
		return nil
	})

	i.PrivateCommand("/balances", "Show balances", func(reply interact.Reply) error {
		reply.Message("Please select an exchange session")
		for name := range it.environment.Sessions() {
			reply.AddButton(name, "session", name)
		}
		return nil
	}).Next(func(sessionName string, reply interact.Reply) error {
		session, ok := it.environment.Session(sessionName)
		if !ok {
			reply.Message(fmt.Sprintf("Session %s not found", sessionName))
			return fmt.Errorf("session %s not found", sessionName)
		}

		message := "Your balances\n"
		balances := session.Account.Balances()
		for _, balance := range balances {
			if balance.Total().IsZero() {
				continue
			}

			message += "- " + balance.String() + "\n"
		}

		reply.Message(message)
		return nil
	})

	i.PrivateCommand("/position", "Show Position", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*PositionReader)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
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

			if position.Base.IsZero() {
				reply.Message(fmt.Sprintf("Strategy %q has no opened position", signature))
				return fmt.Errorf("strategy %T has no opened position", strategy)
			}
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		return nil
	})

	i.PrivateCommand("/closeposition", "Close position", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*PositionCloser)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
			reply.Message("Please choose one strategy")
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

				if position.Base.IsZero() {
					reply.Message("No opened position")
					if kc, ok := reply.(interact.KeyboardController); ok {
						kc.RemoveKeyboard()
					}
					return fmt.Errorf("no opened position")
				}
			}
		}

		reply.Message("Choose or enter the percentage to close")
		for _, p := range []string{"5%", "25%", "50%", "80%", "100%"} {
			reply.AddButton(p, "percentage", p)
		}

		return nil
	}).Next(func(percentageStr string, reply interact.Reply) error {
		percentage, err := fixedpoint.NewFromString(percentageStr)
		if err != nil {
			reply.Message(fmt.Sprintf("%q is not a valid percentage string", percentageStr))
			return err
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		err = it.closePositionContext.closer.ClosePosition(context.Background(), percentage)
		if err != nil {
			reply.Message(fmt.Sprintf("Failed to close the position, %s", err.Error()))
			return err
		}

		reply.Message("Done")
		return nil
	})

	i.PrivateCommand("/status", "Strategy Status", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*StrategyStatusProvider)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports StrategyStatusProvider")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyStatusProvider)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support strategy status provider", signature))
			return fmt.Errorf("strategy %s does not implement StrategyStatusProvider interface", signature)
		}

		status := controller.GetStatus()

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if status == types.StrategyStatusRunning {
			reply.Message(fmt.Sprintf("Strategy %s is running.", signature))
		} else if status == types.StrategyStatusStopped {
			reply.Message(fmt.Sprintf("Strategy %s is not running.", signature))
		}

		return nil
	})

	i.PrivateCommand("/suspend", "Suspend Strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*StrategyController)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports StrategyController")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyController)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support strategy suspend", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController interface", signature)
		}

		// Check strategy status before suspend
		status := controller.GetStatus()
		if status != types.StrategyStatusRunning {
			reply.Message(fmt.Sprintf("Strategy %s is not running.", signature))
			return nil
		}

		err := controller.Suspend(context.Background())

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("Failed to suspend the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s suspended.", signature))
		return nil
	})

	i.PrivateCommand("/resume", "Resume Strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*StrategyController)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports StrategyController")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyController)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support strategy resume", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController interface", signature)
		}

		// Check strategy status before resume
		status := controller.GetStatus()
		if status != types.StrategyStatusStopped {
			reply.Message(fmt.Sprintf("Strategy %s is running.", signature))
			return nil
		}

		err := controller.Resume(context.Background())

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("Failed to resume the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s resumed.", signature))
		return nil
	})

	i.PrivateCommand("/emergencystop", "Emergency Stop", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if buttonsForm, found := it.AddSupportedStrategyButtons((*EmergencyStopper)(nil)); found {
			reply.AddMultipleButtons(buttonsForm)
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports EmergencyStopper")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(EmergencyStopper)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support emergency stop", signature))
			return fmt.Errorf("strategy %s does not implement EmergencyStopper interface", signature)
		}

		err := controller.EmergencyStop(context.Background())

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("Failed to stop the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s stopped and the position closed.", signature))
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
