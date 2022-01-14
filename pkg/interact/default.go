package interact

import "context"

var defaultInteraction = New()

func Default() *Interact {
	return defaultInteraction
}

func SetMessenger(messenger Messenger) {
	defaultInteraction.SetMessenger(messenger)
}

func AddCustomInteraction(custom CustomInteraction) {
	custom.Commands(defaultInteraction)
}

func Start(ctx context.Context) error {
	return defaultInteraction.Start(ctx)
}
