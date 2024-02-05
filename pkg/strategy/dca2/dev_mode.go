package dca2

// DevMode, if Enabled is true. it means it will check the running field
type DevMode struct {
	Enabled      bool `json:"enabled"`
	IsNewAccount bool `json:"isNewAccount"`
}
