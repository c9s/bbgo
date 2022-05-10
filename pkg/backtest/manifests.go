package backtest

import "encoding/json"

type Manifests map[InstancePropertyIndex]string

func (m Manifests) MarshalJSON() ([]byte, error) {
	var arr []interface{}
	for k, v := range m {
		arr = append(arr, map[string]interface{}{
			"id":       k.ID,
			"instance": k.InstanceID,
			"property": k.Property,
			"filename": v,
		})

	}
	return json.MarshalIndent(arr, "", "  ")
}
