package signal

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/c9s/bbgo/pkg/types"
)

var registry = map[string]types.SignalProvider{}
var registryMutex sync.Mutex

// Register registers a signal provider with the given name.
func Register(provider types.SignalProvider) {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	id := provider.ID()
	if _, exists := registry[id]; exists {
		panic(fmt.Errorf("signal provider %s already registered", id))
	}

	registry[id] = provider
}

// GetProvider retrieves a signal provider by its name.
func GetProvider(name string) (types.SignalProvider, bool) {
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
	// Register(&KLineShapeSignal{})
}

// ProviderWrapper is the signal wrapper
type ProviderWrapper struct {
	Weight float64 `json:"weight"`

	Signal types.SignalProvider `json:"-"`
}

type DynamicConfig struct {
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

		configEntry.Signal = providerInstanceRef.Interface().(types.SignalProvider)

		c.Signals = append(c.Signals, configEntry)
	}

	return nil
}

type Config struct {
	Weight float64 `json:"weight"`

	BollingerBandTrendSignal *BollingerBandTrendSignal       `json:"bollingerBandTrend,omitempty"`
	OrderBookBestPriceSignal *OrderBookBestPriceVolumeSignal `json:"orderBookBestPrice,omitempty"`
	DepthRatioSignal         *DepthRatioSignal               `json:"depthRatio,omitempty"`
	KLineShapeSignal         *KLineShapeSignal               `json:"klineShape,omitempty"`
	TradeVolumeWindowSignal  *TradeVolumeWindowSignal        `json:"tradeVolumeWindow,omitempty"`
}

// Get returns the first non-nil signal provider from the config.
func (c *Config) Get() types.SignalProvider {
	if c.OrderBookBestPriceSignal != nil {
		return c.OrderBookBestPriceSignal
	} else if c.DepthRatioSignal != nil {
		return c.DepthRatioSignal
	} else if c.BollingerBandTrendSignal != nil {
		return c.BollingerBandTrendSignal
	} else if c.TradeVolumeWindowSignal != nil {
		return c.TradeVolumeWindowSignal
	}

	panic(fmt.Errorf("no valid signal provider found, please check your config"))
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
