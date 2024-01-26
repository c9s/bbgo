package okex

import (
	"context"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type KLineStream -interface
type KLineStream struct {
	types.StandardStream

	kLineEventCallbacks []func(candle KLineEvent)
}

func NewKLineStream() *KLineStream {
	k := &KLineStream{
		StandardStream: types.NewStandardStream(),
	}

	k.SetParser(parseWebSocketEvent)
	k.SetDispatcher(k.dispatchEvent)
	k.SetEndpointCreator(func(_ context.Context) (string, error) { return okexapi.PublicBusinessWebSocketURL, nil })
	k.SetPingInterval(pingInterval)

	// K line channel is public only
	k.SetPublicOnly()
	k.OnConnect(k.handleConnect)
	k.OnKLineEvent(k.handleKLineEvent)

	return k
}

func (s *KLineStream) handleConnect() {
	var subs []WebsocketSubscription
	for _, subscription := range s.Subscriptions {
		if subscription.Channel != types.KLineChannel {
			continue
		}

		sub, err := convertSubscription(subscription)
		if err != nil {
			log.WithError(err).Errorf("subscription convert error")
			continue
		}

		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		return
	}

	log.Infof("subscribing channels: %+v", subs)
	err := s.Conn.WriteJSON(WebsocketOp{
		Op:   "subscribe",
		Args: subs,
	})

	if err != nil {
		log.WithError(err).Error("subscribe error")
	}
}

func (s *KLineStream) handleKLineEvent(k KLineEvent) {
	for _, event := range k.Events {
		kline := kLineToGlobal(event, types.Interval(k.Interval), k.Symbol)
		if kline.Closed {
			s.EmitKLineClosed(kline)
		} else {
			s.EmitKLine(kline)
		}
	}
}

func (s *KLineStream) dispatchEvent(e interface{}) {
	switch et := e.(type) {
	case *WebSocketEvent:
		if err := et.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
			return
		}

	case *KLineEvent:
		s.EmitKLineEvent(*et)
	}
}
