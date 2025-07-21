package max

import (
	"github.com/google/uuid"

	"github.com/c9s/bbgo/pkg/types"
)

// BBGO is a broker on MAX
const spotBrokerID = "bbgo"

func NewClientOrderID(originalID string, tags ...string) (clientOrderID string) {
	// skip blank client order ID
	if originalID == types.NoClientOrderID {
		return ""
	} else if originalID != "" {
		return originalID
	}

	prefix := "x-" + spotBrokerID + "-"

	for _, tag := range tags {
		prefix += tag + "-"
	}

	clientOrderID = uuid.New().String()
	clientOrderID = prefix + clientOrderID
	if len(clientOrderID) > 36 {
		return clientOrderID[0:36]
	}

	return clientOrderID
}
