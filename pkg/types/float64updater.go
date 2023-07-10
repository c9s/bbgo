package types

//go:generate callbackgen -type Float64Updater
type Float64Updater struct {
	updateCallbacks []func(v float64)
}
