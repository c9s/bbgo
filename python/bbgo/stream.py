import asyncio
from typing import Callable
from typing import List

import grpc

import bbgo_pb2
import bbgo_pb2_grpc
from bbgo.enums import ChannelType
from bbgo.enums import DepthType

from .data import Event
from .data import MarketDataEvent
from .data import Subscription
from .data import UserDataEvent


class Stream(object):
    subscriptions: List[Subscription]

    def __init__(self, host: str, port: int, user_data: bool = False):
        self.host = host
        self.port = port
        self.user_data = user_data

        self.subscriptions = []
        self.event_handlers = []

    def subscribe(self, exchange: str, channel: str, symbol: str, depth: str = None, interval: str = None):
        subscription = Subscription(exchange=exchange, channel=ChannelType(channel), symbol=symbol)

        if depth is not None:
            subscription.depth = DepthType(depth)

        if interval is not None:
            subscription.interval = interval

        self.subscriptions.append(subscription)

    def add_event_handler(self, event_handler: Callable) -> None:
        self.event_handlers.append(event_handler)

    def fire_event_handlers(self, event: Event) -> None:
        for event_handler in self.event_handlers:
            event_handler(event)

    @property
    def address(self):
        return f'{self.host}:{self.port}'

    async def _subscribe(self):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.MarketDataServiceStub(channel)

            request = bbgo_pb2.SubscribeRequest(subscriptions=[s.to_pb() for s in self.subscriptions])
            async for response in stub.Subscribe(request):
                event = MarketDataEvent.from_pb(response)
                self.fire_event_handlers(event)

    async def _subscribe_user_data(self):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.UserDataServiceStub(channel)

            request = bbgo_pb2.Empty()
            async for response in stub.SubscribeUserData(request):
                event = UserDataEvent.from_pb(response)
                self.fire_event_handlers(event)

    def start(self):
        coroutines = [self._subscribe()]
        if self.user_data:
            coroutines.append(self._subscribe_user_data())

        group = asyncio.gather(*coroutines)
        loop = asyncio.get_event_loop()
        loop.run_until_complete(group)
        loop.close()
