package types

func ClassifyOrdersByStatus(orders []Order) (opened, cancelled, filled, unexpected []Order) {
	for _, order := range orders {
		switch order.Status {
		case OrderStatusNew, OrderStatusPartiallyFilled:
			opened = append(opened, order)
		case OrderStatusFilled:
			filled = append(filled, order)
		case OrderStatusCanceled:
			cancelled = append(cancelled, order)
		default:
			unexpected = append(unexpected, order)
		}
	}

	return opened, cancelled, filled, unexpected
}
