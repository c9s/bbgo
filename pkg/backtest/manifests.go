package backtest

import "encoding/json"

type Manifests map[InstancePropertyIndex]string

func (m Manifests) MarshalJSON() ([]byte, error) {
	var arr []interface{}
	for k, v := range m {
		arr = append(arr, map[string]interface{}{
			"type":             "strategyProperty",
			"filename":         v,
			"strategyId":       k.ID,
			"strategyInstance": k.InstanceID,
			"strategyProperty": k.Property,
		})

	}
	return json.MarshalIndent(arr, "", "  ")
}
