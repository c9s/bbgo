package signal

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/c9s/bbgo/pkg/bbgo"
)

var registry = map[string]bbgo.SignalProvider{}
var registryMutex sync.Mutex

// Register registers a signal provider with the given name.
func Register(provider bbgo.SignalProvider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	id := provider.ID()
	if _, exists := registry[id]; exists {
		panic(fmt.Errorf("signal provider %s already registered", id))
	}

	registry[id] = provider
}

// GetProvider retrieves a signal provider by its name.
func GetProvider(name string) (bbgo.SignalProvider, bool) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	provider, exists := registry[name]
	return provider, exists
}

func init() {
	Register(&DepthRatioSignal{})
	Register(&BollingerBandTrendSignal{})
	Register(&OrderBookBestPriceVolumeSignal{})
	Register(&TradeVolumeWindowSignal{})
	Register(&LiquidityDemandSignal{})
	// Register(&KLineShapeSignal{})
}

// ProviderWrapper is the signal wrapper
type ProviderWrapper struct {
	Weight float64 `json:"weight"`

	Signal bbgo.SignalProvider `json:"-"`
}

type DynamicConfig struct {
	// SessionName is the name of the session to use for this signal provider
	SessionName string `json:"session,omitempty"` // optional session name, if not set, it will use the default session

	// Symbol is the symbol to bind the signal provider to
	Symbol string `json:"symbol,omitempty"` // symbol to bind the signal provider to, if not set, it will use the default symbol

	Signals []ProviderWrapper
}

func (c *DynamicConfig) UnmarshalJSON(data []byte) error {
	var configList []map[string]json.RawMessage
	err := json.Unmarshal(data, &configList)
	if err != nil {
		return err
	}

	for _, item := range configList {
		providerId, err := getProviderID(item)
		if err != nil {
			return fmt.Errorf("failed to get provider ID from item: %v", item)
		}

		providerConfigJson, ok := item[providerId]
		if !ok {
			return fmt.Errorf("provider config %s not found in item: %+v", providerId, item)
		}

		providerType, ok := GetProvider(providerId)
		if !ok {
			return fmt.Errorf("signal provider %s not found in registry", providerId)
		}

		// decode common signal config fields
		var configEntry ProviderWrapper
		if err := json.Unmarshal(providerConfigJson, &configEntry); err != nil {
			return fmt.Errorf("failed to decode signal provider config: %w", err)
		}

		providerTypeRef := reflect.TypeOf(providerType)
		providerInstanceRef := reflect.New(providerTypeRef.Elem())

		if err := json.Unmarshal(providerConfigJson, providerInstanceRef.Interface()); err != nil {
			return err
		}

		configEntry.Signal = providerInstanceRef.Interface().(bbgo.SignalProvider)

		c.Signals = append(c.Signals, configEntry)
	}

	return nil
}

func getProviderID(item map[string]json.RawMessage) (string, error) {
	for key := range item {
		switch key {
		case "weight":
		default:
			return key, nil
		}
	}

	return "", errors.New("no valid signal provider found in item")
}
