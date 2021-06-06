package max

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/google/uuid"
)

// BBGO is a broker on MAX
const spotBrokerID = "bbgo"

func NewClientOrderID(originalID string, tags ...string) (clientOrderID string) {
	// skip blank client order ID
	if originalID == types.NoClientOrderID {
		return ""
	}

	prefix := "x-" + spotBrokerID + "-"

	for _, tag := range tags {
		prefix += tag + "-"
	}

	prefixLen := len(prefix)

	if originalID != "" {
		// try to keep the whole original client order ID if user specifies it.
		if prefixLen+len(originalID) > 32 {
			return originalID
		}

		clientOrderID = prefix + originalID
		return clientOrderID
	}

	clientOrderID = uuid.New().String()
	clientOrderID = prefix + clientOrderID
	if len(clientOrderID) > 32 {
		return clientOrderID[0:32]
	}

	return clientOrderID
}
