package signal

type KLineShapeSignal struct {
	FullBodyThreshold float64 `json:"fullBodyThreshold"`
}

func (s *KLineShapeSignal) ID() string {
	return "klineShape"
}
