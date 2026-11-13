package marketdata

import (
	"encoding/json"
	"fmt"
)

// Source type names, as they appear in configuration.
const (
	SourceTypeBinanceCSV = "binanceCsv"
	SourceTypeReplay     = "replay"
	SourceTypeGRPC       = "grpc"
	SourceTypeAmberData  = "amberdata"
)

// SourceConfig declares one market data source in a configuration file.
//
// The provider-specific settings live under Params rather than being inlined,
// because the two encodings bbgo parses disagree about inline maps: yaml
// supports `,inline` and encoding/json does not. An explicit key works in both
// and makes a config readable without knowing which provider owns which field.
//
// This type lives in the core package, and the factory that turns it into a
// Source lives in pkg/marketdata/registry. That split is deliberate: it lets
// pkg/bbgo declare the configuration without importing every provider, and so
// without pulling parquet, grpc and an HTTP client into everything that imports
// pkg/bbgo.
type SourceConfig struct {
	// Type selects the provider. See the SourceType constants.
	Type string `json:"type" yaml:"type"`

	// Name overrides the source name used in logs and merge diagnostics.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Params is the provider's own configuration.
	Params map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// DecodeParams decodes Params into a provider's configuration struct.
//
// It round-trips through JSON, which is how pkg/bbgo already turns a parsed
// yaml stash into a strategy struct, so a provider config can use the same
// json tags as everything else in the codebase.
func (c SourceConfig) DecodeParams(out any) error {
	if len(c.Params) == 0 {
		return nil
	}

	encoded, err := json.Marshal(c.Params)
	if err != nil {
		return fmt.Errorf("marketdata: encoding params of source %q: %w", c.describe(), err)
	}

	if err := json.Unmarshal(encoded, out); err != nil {
		return fmt.Errorf("marketdata: decoding params of source %q: %w", c.describe(), err)
	}

	return nil
}

func (c SourceConfig) describe() string {
	if c.Name != "" {
		return c.Name + " (" + c.Type + ")"
	}
	return c.Type
}

// Describe returns a human-readable identifier for error messages.
func (c SourceConfig) Describe() string { return c.describe() }
