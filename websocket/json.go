package websocket

import (
	"encoding/json"
	"io"

	"github.com/gorilla/websocket"
)

// WriteJSON writes the JSON encoding of v as a message.
//
// See the documentation for encoding/json Marshal for details about the
// conversion of Go values to JSON.
func (c *WebSocketClient) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrConnectionLost
	}
	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	err1 := json.NewEncoder(w).Encode(v)
	err2 := w.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// ReadJSON reads the next JSON-encoded message from the connection and stores
// it in the value pointed to by v.
//
// See the documentation for the encoding/json Unmarshal function for details
// about the conversion of JSON to a Go value.
func (c *WebSocketClient) ReadJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrConnectionLost
	}
	_, r, err := c.conn.NextReader()
	if err != nil {
		return err
	}
	err = json.NewDecoder(r).Decode(v)
	if err == io.EOF {
		// One value is expected in the message.
		err = io.ErrUnexpectedEOF
	}
	return err
}
