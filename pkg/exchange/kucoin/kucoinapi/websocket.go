package kucoinapi

import "encoding/json"

type WebSocketCommand struct {
    Id             int64  `json:"id"`
    Type           string `json:"type"`
    Topic          string `json:"topic"`
    PrivateChannel bool   `json:"privateChannel"`
    Response       bool   `json:"response"`
}

func (c *WebSocketCommand) JSON() ([]byte, error) {
    type tt WebSocketCommand
    var a = (*tt)(c)
    return json.Marshal(a)
}