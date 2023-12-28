package dca2

import (
	"fmt"
	"strings"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

func (s *Strategy) debugOrders(submitOrders []types.Order) {
	var sb strings.Builder
	sb.WriteString("DCA ORDERS[\n")
	for i, order := range submitOrders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("] END OF DCA ORDERS")

	s.logger.Info(sb.String())
}

func debugRoundOrders(logger *logrus.Entry, roundName string, round Round) {
	var sb strings.Builder
	sb.WriteString("ROUND " + roundName + " [\n")
	sb.WriteString(round.TakeProfitOrder.String() + "\n")
	sb.WriteString("------------------------------------------------\n")
	for i, order := range round.OpenPositionOrders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("] END OF ROUND")
	logger.Info(sb.String())
}
