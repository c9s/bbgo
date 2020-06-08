package bbgo

import log "github.com/sirupsen/logrus"

func CalculateAverageCost(trades []Trade) (averageCost float64) {
	var totalCost = 0.0
	var totalQuantity = 0.0
	for _, t := range trades {
		if t.IsBuyer {
			totalCost += t.Price * t.Volume
			totalQuantity += t.Volume
		} else {
			totalCost -= t.Price * t.Volume
			totalQuantity -= t.Volume
		}
	}

	averageCost = totalCost / totalQuantity
	return
}

func CalculateCostAndProfit(trades []Trade, currentPrice float64) (averageBidPrice, stock, profit, fee float64) {
	var bidVolume = 0.0
	var bidAmount = 0.0
	var bidFee = 0.0
	for _, t := range trades {
		if t.IsBuyer {
			bidVolume += t.Volume
			bidAmount += t.Price * t.Volume
			switch t.FeeCurrency {
			case "BTC":
				bidFee += t.Price * t.Fee
			}
		}
	}

	log.Infof("average bid price = (total amount %f + total fee %f) / volume %f", bidAmount, bidFee, bidVolume)
	averageBidPrice = (bidAmount + bidFee) / bidVolume

	var feeRate = 0.001
	var askVolume = 0.0
	var askFee = 0.0
	for _, t := range trades {
		if !t.IsBuyer {
			profit += (t.Price - averageBidPrice) * t.Volume
			askVolume += t.Volume
			switch t.FeeCurrency {
			case "USDT":
				askFee += t.Fee
			}
		}
	}

	profit -= askFee

	stock = bidVolume - askVolume
	futureFee := 0.0
	if stock > 0 {
		stockfee := currentPrice * feeRate * stock
		profit += (currentPrice-averageBidPrice)*stock - stockfee
		futureFee += stockfee
	}

	fee = bidFee + askFee + futureFee
	return
}
