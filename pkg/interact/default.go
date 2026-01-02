package interact

import (
	"context"
	"fmt"
)

var defaultInteraction = New()

func Default() *Interact {
	return defaultInteraction
}

func AddMessenger(messenger Messenger) {
	defaultInteraction.AddMessenger(messenger)
}

func GetMessengers() []Messenger {
	return defaultInteraction.GetMessengers()
}

func AddCustomInteraction(custom CustomInteraction) {
	defaultInteraction.AddCustomInteraction(custom)
}

func Start(ctx context.Context) error {
	return defaultInteraction.Start(ctx)
}

// default interactive dispatcher
var defaultDispatcher *InteractiveMessageDispatcher

// GetDispatcher returns the default interactive message dispatcher
// NOTE: the default dispatcher is configured only dispatching interation callbacks by the first Slack menssenger.
// If you have multiple Slack messengers, you need to create your own dispatcher.
func GetDispatcher() (*InteractiveMessageDispatcher, error) {
	if defaultDispatcher != nil {
		return defaultDispatcher, nil
	}
	var slackMessenger *Slack
	for _, messenger := range Default().GetMessengers() {
		if sm, ok := messenger.(*Slack); ok {
			slackMessenger = sm
			break
		}
	}
	if slackMessenger == nil {
		return nil, fmt.Errorf("slack interaction is not enabled")
	}
	defaultDispatcher = NewInteractiveMessageDispatcher(slackMessenger)
	return defaultDispatcher, nil
}
