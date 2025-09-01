package hyperapi

type SignatureResult struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

type InfoReqType string

const (
	Meta     InfoReqType = "meta"
	SpotMeta InfoReqType = "spotMeta"
)
