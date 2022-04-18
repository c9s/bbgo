package bbgo

import (
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/types"
)

type closePositionContext struct {
	signature  string
	controller StrategyControllerInterface
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

func (it *CoreInteraction) GetStrategyController(strategy SingleExchangeStrategy) (controller StrategyControllerInterface, found bool) {
	if strategyController := reflect.ValueOf(&strategy).Elem().FieldByName("strategyController"); strategyController.IsValid() {
		controller = strategyController.Interface().(StrategyControllerInterface)
		return controller, true
	} else {
		return nil, false
	}
}

func (it *CoreInteraction) FilterStrategyByCallBack(callBack string) (strategies []string, found bool) {
	found = false
	for signature, strategy := range it.exchangeStrategies {
		if strategyController, ok := it.GetStrategyController(strategy); ok {
			if strategyController.HasCallback(callBack) {
				strategies = append(strategies, signature)
				found = true
			}
		}
	}

	return strategies, found
}

func GenerateStrategyButtonsForm(strategies []string) [][3]string {
	var buttonsForm [][3]string
	for _, strategy := range strategies {
		buttonsForm = append(buttonsForm, [3]string{strategy, "strategy", strategy})
	}

	return buttonsForm
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
		balances := session.GetAccount().Balances()
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
		if strategies, found := it.FilterStrategyByCallBack("GetPositionCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports GetPosition")
		}
		return nil
	}).Cycle(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		position, err := controller.EmitGetPosition()

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("unable to get position of strategy %s: %s", signature, err.Error()))
			return fmt.Errorf("unable to get position of strategy %s: %s", signature, err.Error())
		} else {
			if position != nil {
				reply.Send("Your current position:")
				reply.Send(position.PlainText())

				if position.Base.IsZero() {
					reply.Message(fmt.Sprintf("Strategy %q has no opened position", signature))
					return fmt.Errorf("strategy %T has no opened position", strategy)
				}
			}
		}

		return nil
	})

	i.PrivateCommand("/closeposition", "Close position", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if strategies, found := it.FilterStrategyByCallBack("ClosePositionCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No any strategy supports ClosePosition")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		it.closePositionContext.controller = controller
		it.closePositionContext.signature = signature

		if position, err := controller.EmitGetPosition(); err == nil {
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

		err = it.closePositionContext.controller.EmitClosePosition(percentage)
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
		if strategies, found := it.FilterStrategyByCallBack("GetStatusCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose a strategy")
		} else {
			reply.Message("No strategy supports GetStatus")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		status, err := controller.EmitGetStatus()

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("unable to get status of strategy %s: %s", signature, err.Error()))
			return fmt.Errorf("unable to get status of strategy %s: %s", signature, err.Error())
		} else {
			if status == types.StrategyStatusRunning {
				reply.Message(fmt.Sprintf("Strategy %s is running.", signature))
			} else if status == types.StrategyStatusStopped {
				reply.Message(fmt.Sprintf("Strategy %s is not running.", signature))
			}
		}

		return nil
	})

	i.PrivateCommand("/suspend", "Suspend Strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if strategies, found := it.FilterStrategyByCallBack("SuspendCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports Suspend")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		// Check strategy status before suspend
		status, err := controller.EmitGetStatus()
		if status != types.StrategyStatusRunning {
			reply.Message(fmt.Sprintf("Strategy %s is not running.", signature))
			return nil
		}

		err = controller.EmitSuspend()

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
		if strategies, found := it.FilterStrategyByCallBack("ResumeCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports Resume")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		// Check strategy status before resume
		status, err := controller.EmitGetStatus()
		if status != types.StrategyStatusStopped {
			reply.Message(fmt.Sprintf("Strategy %s is running.", signature))
			return nil
		}

		err = controller.EmitResume()

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
		if strategies, found := it.FilterStrategyByCallBack("EmergencyStopCallback"); found {
			reply.AddMultipleButtons(GenerateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports EmergencyStop")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := it.GetStrategyController(strategy)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyController", signature))
			return fmt.Errorf("strategy %s does not implement StrategyController", signature)
		}

		err := controller.EmitEmergencyStop()

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err != nil {
			reply.Message(fmt.Sprintf("Failed to emergency stop the strategy, %s", err.Error()))
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
