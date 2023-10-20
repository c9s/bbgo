package bitget

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	bookEventCallbacks []func(o BookEvent)
}

func NewStream() *Stream {
	stream := &Stream{
		StandardStream: types.NewStandardStream(),
	}

	stream.SetEndpointCreator(stream.createEndpoint)
	stream.SetParser(parseWebSocketEvent)
	stream.SetDispatcher(stream.dispatchEvent)
	stream.OnConnect(stream.handlerConnect)

	stream.OnBookEvent(stream.handleBookEvent)
	return stream
}

func (s *Stream) syncSubscriptions(opType WsEventType) error {
	if opType != WsEventUnsubscribe && opType != WsEventSubscribe {
		return fmt.Errorf("unexpected subscription type: %v", opType)
	}

	logger := log.WithField("opType", opType)
	args := []WsArg{}
	for _, subscription := range s.Subscriptions {
		arg, err := convertSubscription(subscription)
		if err != nil {
			logger.WithError(err).Errorf("convert error, subscription: %+v", subscription)
			return err
		}

		args = append(args, arg)
	}

	logger.Infof("%s channels: %+v", opType, args)
	if err := s.Conn.WriteJSON(WsOp{
		Op:   opType,
		Args: args,
	}); err != nil {
		logger.WithError(err).Error("failed to send request")
		return err
	}

	return nil
}

func (s *Stream) Unsubscribe() {
	// errors are handled in the syncSubscriptions, so they are skipped here.
	_ = s.syncSubscriptions(WsEventUnsubscribe)
	s.Resubscribe(func(old []types.Subscription) (new []types.Subscription, err error) {
		// clear the subscriptions
		return []types.Subscription{}, nil
	})
}

func (s *Stream) createEndpoint(_ context.Context) (string, error) {
	var url string
	if s.PublicOnly {
		url = bitgetapi.PublicWebSocketURL
	} else {
		url = bitgetapi.PrivateWebSocketURL
	}
	return url, nil
}

func (s *Stream) dispatchEvent(event interface{}) {
	switch e := event.(type) {
	case *WsEvent:
		if err := e.IsValid(); err != nil {
			log.Errorf("invalid event: %v", err)
		}

	case *BookEvent:
		s.EmitBookEvent(*e)
	}
}

func (s *Stream) handlerConnect() {
	if s.PublicOnly {
		// errors are handled in the syncSubscriptions, so they are skipped here.
		_ = s.syncSubscriptions(WsEventSubscribe)
	} else {
		log.Error("*** PRIVATE API NOT IMPLEMENTED ***")
	}
}

func (s *Stream) handleBookEvent(o BookEvent) {
	for _, book := range o.ToGlobalOrderBooks() {
		switch o.Type {
		case ActionTypeSnapshot:
			s.EmitBookSnapshot(book)

		case ActionTypeUpdate:
			s.EmitBookUpdate(book)
		}
	}
}

func convertSubscription(sub types.Subscription) (WsArg, error) {
	arg := WsArg{
		// support spot only
		InstType: instSp,
		Channel:  "",
		InstId:   sub.Symbol,
	}

	switch sub.Channel {
	case types.BookChannel:
		arg.Channel = ChannelOrderBook5

		switch sub.Options.Depth {
		case types.DepthLevel15:
			arg.Channel = ChannelOrderBook15
		case types.DepthLevel200:
			log.Warn("*** The subscription events for the order book may return fewer than 200 bids/asks at a depth of 200. ***")
			arg.Channel = ChannelOrderBook
		}
		return arg, nil
	}

	return arg, fmt.Errorf("unsupported stream channel: %s", sub.Channel)
}

func parseWebSocketEvent(in []byte) (interface{}, error) {
	var event WsEvent

	err := json.Unmarshal(in, &event)
	if err != nil {
		return nil, err
	}

	if event.IsOp() {
		return &event, nil
	}

	switch event.Arg.Channel {
	case ChannelOrderBook, ChannelOrderBook5, ChannelOrderBook15:
		var book BookEvent
		err = json.Unmarshal(event.Data, &book.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal data into BookEvent, Arg: %+v Data: %s, err: %w", event.Arg, string(event.Data), err)
		}

		book.Type = event.Action
		book.InstId = event.Arg.InstId
		return &book, nil
	}

	return nil, fmt.Errorf("unhandled websocket event: %+v", string(in))
}
