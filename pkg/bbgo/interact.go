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

type closePositionContext struct {
	signature  string
	closer     PositionCloser
	percentage fixedpoint.Value
}

type modifyPositionContext struct {
	signature string
	modifier  *types.Position
	target    string
	value     fixedpoint.Value
}

type CoreInteraction struct {
	environment *Environment
	trader      *Trader

	exchangeStrategies    map[string]SingleExchangeStrategy
	closePositionContext  closePositionContext
	modifyPositionContext modifyPositionContext
}

func NewCoreInteraction(environment *Environment, trader *Trader) *CoreInteraction {
	return &CoreInteraction{
		environment:        environment,
		trader:             trader,
		exchangeStrategies: make(map[string]SingleExchangeStrategy),
	}
}

type SimpleInteraction struct {
	Command     string
	Description string
	F           interface{}
	Cmd         *interact.Command
}

func (it *SimpleInteraction) Commands(i *interact.Interact) {
	it.Cmd = i.PrivateCommand(it.Command, it.Description, it.F)
}

func RegisterCommand(command, desc string, f interface{}) *interact.Command {
	it := &SimpleInteraction{
		Command:     command,
		Description: desc,
		F:           f,
	}
	interact.AddCustomInteraction(it)
	return it.Cmd
}

func getStrategySignatures(exchangeStrategies map[string]SingleExchangeStrategy) []string {
	var strategies []string
	for signature := range exchangeStrategies {
		strategies = append(strategies, signature)
	}

	return strategies
}

func filterStrategyByInterface(checkInterface interface{}, exchangeStrategies map[string]SingleExchangeStrategy) (strategies map[string]SingleExchangeStrategy, found bool) {
	found = false
	strategies = make(map[string]SingleExchangeStrategy)
	rt := reflect.TypeOf(checkInterface).Elem()
	for signature, strategy := range exchangeStrategies {
		if ok := reflect.TypeOf(strategy).Implements(rt); ok {
			strategies[signature] = strategy
			found = true
		}
	}

	return strategies, found
}

func filterStrategyByField(fieldName string, fieldType reflect.Type, exchangeStrategies map[string]SingleExchangeStrategy) (strategies map[string]SingleExchangeStrategy, found bool) {
	found = false
	strategies = make(map[string]SingleExchangeStrategy)
	for signature, strategy := range exchangeStrategies {
		r := reflect.ValueOf(strategy).Elem()
		f := r.FieldByName(fieldName)
		if !f.IsZero() && f.Type() == fieldType {
			strategies[signature] = strategy
			found = true
		}
	}

	return strategies, found
}

func generateStrategyButtonsForm(strategies map[string]SingleExchangeStrategy) [][3]string {
	var buttonsForm [][3]string
	signatures := getStrategySignatures(strategies)
	for _, signature := range signatures {
		buttonsForm = append(buttonsForm, [3]string{signature, "strategy", signature})
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
		if strategies, found := filterStrategyByInterface((*PositionReader)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports PositionReader")
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
		if strategies, found := filterStrategyByInterface((*PositionCloser)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports PositionCloser")
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
		if strategies, found := filterStrategyByInterface((*StrategyStatusReader)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose a strategy")
		} else {
			reply.Message("No strategy supports StrategyStatusReader")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyStatusReader)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyStatusReader", signature))
			return fmt.Errorf("strategy %s does not implement StrategyStatusReader", signature)
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
		if strategies, found := filterStrategyByInterface((*StrategyToggler)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports StrategyToggler")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyToggler)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyToggler", signature))
			return fmt.Errorf("strategy %s does not implement StrategyToggler", signature)
		}

		// Check strategy status before suspend
		if controller.GetStatus() != types.StrategyStatusRunning {
			reply.Message(fmt.Sprintf("Strategy %s is not running.", signature))
			return nil
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err := controller.Suspend(); err != nil {
			reply.Message(fmt.Sprintf("Failed to suspend the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s suspended.", signature))
		return nil
	})

	i.PrivateCommand("/resume", "Resume Strategy", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if strategies, found := filterStrategyByInterface((*StrategyToggler)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports StrategyToggler")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(StrategyToggler)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support StrategyToggler", signature))
			return fmt.Errorf("strategy %s does not implement StrategyToggler", signature)
		}

		// Check strategy status before suspend
		if controller.GetStatus() != types.StrategyStatusStopped {
			reply.Message(fmt.Sprintf("Strategy %s is running.", signature))
			return nil
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err := controller.Resume(); err != nil {
			reply.Message(fmt.Sprintf("Failed to resume the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s resumed.", signature))
		return nil
	})

	i.PrivateCommand("/emergencystop", "Emergency Stop", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if strategies, found := filterStrategyByInterface((*EmergencyStopper)(nil), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports EmergencyStopper")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		controller, implemented := strategy.(EmergencyStopper)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support EmergencyStopper", signature))
			return fmt.Errorf("strategy %s does not implement EmergencyStopper", signature)
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if err := controller.EmergencyStop(); err != nil {
			reply.Message(fmt.Sprintf("Failed to emergency stop the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Strategy %s stopped and the position closed.", signature))
		return nil
	})

	// Position updater
	i.PrivateCommand("/modifyposition", "Modify Strategy Position", func(reply interact.Reply) error {
		// it.trader.exchangeStrategies
		// send symbol options
		if strategies, found := filterStrategyByField("Position", reflect.TypeOf(types.NewPosition("", "", "")), it.exchangeStrategies); found {
			reply.AddMultipleButtons(generateStrategyButtonsForm(strategies))
			reply.Message("Please choose one strategy")
		} else {
			reply.Message("No strategy supports Position Modify")
		}
		return nil
	}).Next(func(signature string, reply interact.Reply) error {
		strategy, ok := it.exchangeStrategies[signature]
		if !ok {
			reply.Message("Strategy not found")
			return fmt.Errorf("strategy %s not found", signature)
		}

		r := reflect.ValueOf(strategy).Elem()
		f := r.FieldByName("Position")
		positionModifier, implemented := f.Interface().(*types.Position)
		if !implemented {
			reply.Message(fmt.Sprintf("Strategy %s does not support Position Modify", signature))
			return fmt.Errorf("strategy %s does not implement Position Modify", signature)
		}

		it.modifyPositionContext.modifier = positionModifier
		it.modifyPositionContext.signature = signature

		reply.Message("Please choose what you want to change")
		reply.AddButton("base", "Base", "base")
		reply.AddButton("quote", "Quote", "quote")
		reply.AddButton("cost", "Average Cost", "cost")

		return nil
	}).Next(func(target string, reply interact.Reply) error {
		if target != "base" && target != "quote" && target != "cost" {
			reply.Message(fmt.Sprintf("%q is not a valid target string", target))
			return fmt.Errorf("%q is not a valid target string", target)
		}

		it.modifyPositionContext.target = target

		reply.Message("Enter the amount to change")

		return nil
	}).Next(func(valueStr string, reply interact.Reply) error {
		value, err := fixedpoint.NewFromString(valueStr)
		if err != nil {
			reply.Message(fmt.Sprintf("%q is not a valid value string", valueStr))
			return err
		}

		if kc, ok := reply.(interact.KeyboardController); ok {
			kc.RemoveKeyboard()
		}

		if it.modifyPositionContext.target == "base" {
			err = it.modifyPositionContext.modifier.ModifyBase(value)
		} else if it.modifyPositionContext.target == "quote" {
			err = it.modifyPositionContext.modifier.ModifyQuote(value)
		} else if it.modifyPositionContext.target == "cost" {
			err = it.modifyPositionContext.modifier.ModifyAverageCost(value)
		}

		if err != nil {
			reply.Message(fmt.Sprintf("Failed to modify position of the strategy, %s", err.Error()))
			return err
		}

		reply.Message(fmt.Sprintf("Position of strategy %s modified.", it.modifyPositionContext.signature))
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

// getStrategySignature returns strategy instance unique signature
func getStrategySignature(strategy SingleExchangeStrategy) (string, error) {
	// Returns instance ID
	var signature = callID(strategy)
	if signature != "" {
		return signature, nil
	}

	// Use reflect to build instance signature
	rv := reflect.ValueOf(strategy).Elem()
	if rv.Kind() != reflect.Struct {
		return "", fmt.Errorf("strategy %T instance is not a struct", strategy)
	}

	signature = path.Base(rv.Type().PkgPath())
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldName := rv.Type().Field(i).Name
		if field.Kind() == reflect.String && fieldName != "Status" {
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
