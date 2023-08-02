package bybit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	// Bybit: To avoid network or program issues, we recommend that you send the ping heartbeat packet every 20 seconds
	// to maintain the WebSocket connection.
	pingInterval = 20 * time.Second
)

type Stream struct {
	types.StandardStream
}

func NewStream() *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(stream.parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.SetHeartBeat(stream.ping)

	return stream
}

func (s *Stream) createEndpoint(_ context.Context) (string, error) {
	var url string
	if s.PublicOnly {
		url = bybitapi.WsSpotPublicSpotUrl
	} else {
		url = bybitapi.WsSpotPrivateUrl
	}
	return url, nil
}

func (s *Stream) dispatchEvent(event interface{}) {
	switch e := event.(type) {
	case *WebSocketEvent:
		if err := e.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
		}
	}
}

func (s *Stream) parseWebSocketEvent(in []byte) (interface{}, error) {
	var resp WebSocketEvent
	return &resp, json.Unmarshal(in, &resp)
}

// ping implements the Bybit text message of WebSocket PingPong.
func (s *Stream) ping(ctx context.Context, conn *websocket.Conn, cancelFunc context.CancelFunc) {
	defer func() {
		log.Debug("[bybit] ping worker stopped")
		cancelFunc()
	}()

	var pingTicker = time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-s.CloseC:
			return

		case <-pingTicker.C:
			// it's just for maintaining the liveliness of the connection, so comment out ReqId.
			err := conn.WriteJSON(struct {
				//ReqId string `json:"req_id"`
				Op WsOpType `json:"op"`
			}{
				//ReqId: uuid.NewString(),
				Op: WsOpTypePing,
			})
			if err != nil {
				log.WithError(err).Error("ping error", err)
				s.Reconnect()
				return
			}
		}
	}
}
