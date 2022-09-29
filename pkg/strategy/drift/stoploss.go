package drift

func (s *Strategy) CheckStopLoss() bool {
	stoploss := s.StopLoss.Float64()
	atr := s.atr.Last()
	if s.UseStopLoss {
		if s.sellPrice > 0 && s.sellPrice*(1.+stoploss) <= s.highestPrice ||
			s.buyPrice > 0 && s.buyPrice*(1.-stoploss) >= s.lowestPrice {
			return true
		}
	}
	if s.UseAtr {
		if s.sellPrice > 0 && s.sellPrice+atr <= s.highestPrice ||
			s.buyPrice > 0 && s.buyPrice-atr >= s.lowestPrice {
			return true
		}
	}
	return false
}
