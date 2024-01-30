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
	subscribe(s.Conn, subs)
}

func (s *KLineStream) Connect(ctx context.Context) error {
	if len(s.StandardStream.Subscriptions) == 0 {
		log.Info("no subscriptions in kline")
		return nil
	}
	return s.StandardStream.Connect(ctx)
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

func (s *KLineStream) Unsubscribe() {
	// errors are handled in the syncSubscriptions, so they are skipped here.
	if len(s.StandardStream.Subscriptions) != 0 {
		_ = syncSubscriptions(s.StandardStream.Conn, s.StandardStream.Subscriptions, WsEventTypeUnsubscribe)
	}
	s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
		// clear the subscriptions
		return []types.Subscription{}, nil
	})
}
