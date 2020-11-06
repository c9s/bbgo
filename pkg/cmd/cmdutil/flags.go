package cmdutil

import "github.com/spf13/pflag"

// PersistentFlags defines the flags for environments
func PersistentFlags(flags *pflag.FlagSet) {
	flags.String("binance-api-key", "", "binance api key")
	flags.String("binance-api-secret", "", "binance api secret")
	flags.String("max-api-key", "", "max api key")
	flags.String("max-api-secret", "", "max api secret")
}
