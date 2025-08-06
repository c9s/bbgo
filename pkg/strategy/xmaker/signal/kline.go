package signal

type KLineShapeSignal struct {
	BaseProvider
	Logger

	FullBodyThreshold float64 `json:"fullBodyThreshold"`
}

func (s *KLineShapeSignal) ID() string {
	return "klineShape"
}
