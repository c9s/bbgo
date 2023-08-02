package bybit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseWebSocketEvent(t *testing.T) {
	t.Run("[public] PingEvent without req id", func(t *testing.T) {
		s := NewStream()
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		e, ok := raw.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   nil,
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[public] PingEvent with req id", func(t *testing.T) {
		s := NewStream()
		msg := `{"success":true,"ret_msg":"pong","conn_id":"a806f6c4-3608-4b6d-a225-9f5da975bc44","req_id":"b26704da-f5af-44c2-bdf7-935d6739e1a0","op":"ping"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"
		e, ok := raw.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ConnId:  "a806f6c4-3608-4b6d-a225-9f5da975bc44",
			ReqId:   &expReqId,
			Op:      WsOpTypePing,
			Args:    nil,
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent without req id", func(t *testing.T) {
		s := NewStream()
		msg := `{"op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		e, ok := raw.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketEvent{
			Success: nil,
			RetMsg:  nil,
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   nil,
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})

	t.Run("[private] PingEvent with req id", func(t *testing.T) {
		s := NewStream()
		msg := `{"req_id":"78d36b57-a142-47b7-9143-5843df77d44d","op":"pong","args":["1690884539181"],"conn_id":"civn4p1dcjmtvb69ome0-yrt1"}`
		raw, err := s.parseWebSocketEvent([]byte(msg))
		assert.NoError(t, err)

		expReqId := "78d36b57-a142-47b7-9143-5843df77d44d"
		e, ok := raw.(*WebSocketEvent)
		assert.True(t, ok)
		assert.Equal(t, &WebSocketEvent{
			Success: nil,
			RetMsg:  nil,
			ConnId:  "civn4p1dcjmtvb69ome0-yrt1",
			ReqId:   &expReqId,
			Op:      WsOpTypePong,
			Args:    []string{"1690884539181"},
		}, e)

		assert.NoError(t, e.IsValid())
	})
}

func Test_WebSocketEventIsValid(t *testing.T) {
	t.Run("[public] valid op ping", func(t *testing.T) {
		expSucceeds := true
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[private] valid op ping", func(t *testing.T) {
		w := &WebSocketEvent{
			Success: nil,
			RetMsg:  nil,
			ReqId:   nil,
			ConnId:  "test-conndid",
			Op:      WsOpTypePong,
			Args:    nil,
		}
		assert.NoError(t, w.IsValid())
	})

	t.Run("[public] un-Success", func(t *testing.T) {
		expSucceeds := false
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %#v", w), w.IsValid())
	})

	t.Run("[public] missing Success field", func(t *testing.T) {
		expRetMsg := string(WsOpTypePong)
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketEvent{
			RetMsg: &expRetMsg,
			ReqId:  &expReqId,
			ConnId: "test-conndid",
			Op:     WsOpTypePing,
			Args:   nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %#v", w), w.IsValid())
	})

	t.Run("[public] invalid ret msg", func(t *testing.T) {
		expSucceeds := false
		expRetMsg := "PINGPONGPINGPONG"
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketEvent{
			Success: &expSucceeds,
			RetMsg:  &expRetMsg,
			ReqId:   &expReqId,
			ConnId:  "test-conndid",
			Op:      WsOpTypePing,
			Args:    nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %#v", w), w.IsValid())
	})

	t.Run("[public] missing RetMsg field", func(t *testing.T) {
		expReqId := "b26704da-f5af-44c2-bdf7-935d6739e1a0"

		w := &WebSocketEvent{
			ReqId:  &expReqId,
			ConnId: "test-conndid",
			Op:     WsOpTypePing,
			Args:   nil,
		}
		assert.Error(t, fmt.Errorf("unexpeted response of pong: %#v", w), w.IsValid())
	})

	t.Run("unexpected op type", func(t *testing.T) {
		w := &WebSocketEvent{
			Op: WsOpType("unexpected"),
		}
		assert.Error(t, fmt.Errorf("unexpected op type: %#v", w), w.IsValid())
	})
}
