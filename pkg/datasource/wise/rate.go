package wise

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Rate struct {
	Value  fixedpoint.Value `json:"rate"`
	Target string           `json:"target"`
	Source string           `json:"source"`
	Time   Time             `json:"time"`
}
