package types

import (
	"testing"
	"time"
)

func TestConnectivity(t *testing.T) {
	t.Run("general", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()
		conn1.setAuthed()
		conn1.setDisconnect()
	})

	t.Run("reconnect", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()
		conn1.setAuthed()
		conn1.setDisconnect()

		conn1.setConnect()
		conn1.setAuthed()
		conn1.setDisconnect()
	})

	t.Run("no-auth reconnect", func(t *testing.T) {
		conn1 := NewConnectivity()
		conn1.setConnect()
		conn1.setDisconnect()

		conn1.setConnect()
		conn1.setDisconnect()
	})
}

func waitSigChan(c <-chan struct{}, timeoutDuration time.Duration) bool {
	select {
	case <-time.After(timeoutDuration):
		return false

	case <-c:
		return true
	}
}
