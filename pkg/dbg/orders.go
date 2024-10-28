package dbg

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	types2 "github.com/c9s/bbgo/pkg/types"
)

func DebugSubmitOrders(logger logrus.FieldLogger, submitOrders []types2.SubmitOrder) {
	var sb strings.Builder
	sb.WriteString("SubmitOrders[\n")
	for i, order := range submitOrders {
		sb.WriteString(fmt.Sprintf("%3d) ", i+1) + order.String() + "\n")
	}
	sb.WriteString("] End of SubmitOrders")

	logger.Info(sb.String())
}
