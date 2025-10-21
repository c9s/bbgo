package coinbase

import (
	"encoding/json"
	"strings"

	"github.com/c9s/requestgen"
)

func isNotFoundError(err error) bool {
	if errResp, ok := err.(*requestgen.ErrResponse); ok {
		var body struct {
			Message string `json:"message"`
		}
		err2 := json.Unmarshal(errResp.Body, &body)
		if err2 == nil && strings.Contains(body.Message, "not found") {
			return true
		}
	}
	return false
}
