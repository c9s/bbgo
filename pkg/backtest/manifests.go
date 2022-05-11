package backtest

import "encoding/json"

type ManifestEntry struct {
	Type             string `json:"type"`
	Filename         string `json:"filename"`
	StrategyID       string `json:"strategyID"`
	StrategyInstance string `json:"strategyInstance"`
	StrategyProperty string `json:"strategyProperty"`
}

type Manifests map[InstancePropertyIndex]string

func (m Manifests) MarshalJSON() ([]byte, error) {
	var arr []ManifestEntry
	for k, v := range m {
		arr = append(arr, ManifestEntry{
			Type:             "strategyProperty",
			Filename:         v,
			StrategyID:       k.ID,
			StrategyInstance: k.InstanceID,
			StrategyProperty: k.Property,
		})

	}
	return json.MarshalIndent(arr, "", "  ")
}
