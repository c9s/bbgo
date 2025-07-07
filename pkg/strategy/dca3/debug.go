package dca3

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

func debugOrders(logger *logrus.Entry, name string, orders []types.Order) {
	var sb strings.Builder
	sb.WriteString(name + " ORDERS[\n")
	for i, order := range orders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("] END OF " + name + " ORDERS")

	logger.Info(sb.String())
}

func debugRoundOrders(logger *logrus.Entry, roundName string, round Round) {
	var sb strings.Builder
	sb.WriteString("ROUND " + roundName + " [\n")
	for i, order := range round.TakeProfitOrders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("------------------------------------------------\n")
	for i, order := range round.OpenPositionOrders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("] END OF ROUND")
	logger.Info(sb.String())
}
