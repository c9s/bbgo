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
	defaultInteraction.AddCustomInteraction(custom)
}

func Start(ctx context.Context) error {
	return defaultInteraction.Start(ctx)
}
