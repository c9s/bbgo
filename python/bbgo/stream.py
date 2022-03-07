import asyncio
from typing import Callable
from typing import List

import grpc

from . import bbgo_pb2
from . import bbgo_pb2_grpc


class Stream(object):

    def __init__(self, host: str, port: int, subscriptions: List[bbgo_pb2.Subscription]):
        self.host = host
        self.port = port
        self.subscriptions = subscriptions

        # callbacks for public channel
        self.book_event_callbacks = []
        self.trade_event_callbacks = []
        self.ticker_event_callbacks = []

        # callbacks for private channel
        self.order_snapshot_event_callbacks = []
        self.order_update_event_callbacks = []
        self.trade_snapshot_event_callbacks = []
        self.trade_update_event_callbacks = []
        self.account_snapshot_event_callbacks = []
        self.account_update_event_callbacks = []

    @property
    def address(self):
        return f'{self.host}:{self.port}'

    async def subscribe(self):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.BBGOStub(channel)

            request = bbgo_pb2.SubscribeRequest(subscriptions=self.subscriptions)
            async for response in stub.Subcribe(request):
                self.dispatch(response)

    async def subscribe_user_data(self):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.BBGOStub(channel)

            request = bbgo_pb2.Empty()
            async for response in stub.SubcribeUserData(request):
                self.dispatch_user_events(response)

    def start(self):
        group = asyncio.gather(
            self.subscribe(),
            self.subscribe_user_data(),
        )
        loop = asyncio.get_event_loop()
        loop.run_until_complete(group)
        loop.close()

    def dispatch(self, response: bbgo_pb2.SubscribeResponse):
        m = {
            bbgo_pb2.Channel.BOOK: self.emit_book_event_callbacks,
            bbgo_pb2.Channel.TRADE: self.emit_trade_event_callbacks,
            bbgo_pb2.Channel.TICKER: self.emit_ticker_event_callbacks,
            bbgo_pb2.Channel.USER: self.dispatch_user_events,
        }
        m[response.channel](response)

    def dispatch_user_events(self, response: bbgo_pb2.SubscribeResponse):
        m = {
            bbgo_pb2.Event.ORDER_SNAPSHOT: self.emit_order_snapshot_event_callbacks,
            bbgo_pb2.Event.ORDER_UPDATE: self.emit_order_update_event_callbacks,
            bbgo_pb2.Event.TRADE_SNAPSHOT: self.emit_trade_snapshot_event_callbacks,
            bbgo_pb2.Event.TRADE_UPDATE: self.emit_trade_update_event_callbacks,
            bbgo_pb2.Event.ACCOUNT_SNAPSHOT: self.emit_account_snapshot_event_callbacks,
            bbgo_pb2.Event.ACCOUNT_UPDATE: self.emit_account_update_event_callbacks,
        }
        m[response.event](response)

    def on_book_event(self, callback: Callable) -> None:
        self.book_event_callbacks.append(callback)

    def emit_book_event_callbacks(self, event) -> None:
        for callback in self.book_event_callbacks:
            callback(event)

    def on_trade_event(self, callback: Callable) -> None:
        self.trade_event_callbacks.append(callback)

    def emit_trade_event_callbacks(self, event) -> None:
        for callback in self.trade_event_callbacks:
            callback(event)

    def on_ticker_event(self, callback: Callable) -> None:
        self.ticker_event_callbacks.append(callback)

    def emit_ticker_event_callbacks(self, event) -> None:
        for callback in self.ticker_event_callbacks:
            callback(event)

    def on_order_snapshot_event(self, callback: Callable) -> None:
        self.order_snapshot_event_callbacks.append(callback)

    def emit_order_snapshot_event_callbacks(self, event) -> None:
        for callback in self.order_snapshot_event_callbacks:
            callback(event)

    def on_order_update_event(self, callback: Callable) -> None:
        self.order_update_event_callbacks.append(callback)

    def emit_order_update_event_callbacks(self, event) -> None:
        for callback in self.order_update_event_callbacks:
            callback(event)

    def on_trade_snapshot_event(self, callback: Callable) -> None:
        self.trade_snapshot_event_callbacks.append(callback)

    def emit_trade_snapshot_event_callbacks(self, event) -> None:
        for callback in self.trade_snapshot_event_callbacks:
            callback(event)

    def on_trade_update_event(self, callback: Callable) -> None:
        self.trade_update_event_callbacks.append(callback)

    def emit_trade_update_event_callbacks(self, event) -> None:
        for callback in self.trade_update_event_callbacks:
            callback(event)

    def on_account_snapshot_event(self, callback: Callable) -> None:
        self.account_snapshot_event_callbacks.append(callback)

    def emit_account_snapshot_event_callbacks(self, event) -> None:
        for callback in self.account_snapshot_event_callbacks:
            callback(event)

    def on_account_update_event(self, callback: Callable) -> None:
        self.account_update_event_callbacks.append(callback)

    def emit_account_update_event_callbacks(self, event) -> None:
        for callback in self.account_update_event_callbacks:
            callback(event)
