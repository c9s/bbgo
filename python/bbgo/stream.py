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

    def __init__(self, host: str, port: int):
        self.host = host
        self.port = port

        self.subscriptions = []
        self.sessions = []
        self.event_handlers = []

    def subscribe(self, exchange: str, channel: str, symbol: str, depth: str = None, interval: str = None):
        subscription = Subscription(exchange=exchange, channel=ChannelType.from_str(channel), symbol=symbol)

        if depth is not None:
            subscription.depth = DepthType(depth)

        if interval is not None:
            subscription.interval = interval

        self.subscriptions.append(subscription)

    def subscribe_user_data(self, session: str):
        self.sessions.append(session)

    def add_event_handler(self, event_handler: Callable) -> None:
        self.event_handlers.append(event_handler)

    def fire_event_handlers(self, event: Event) -> None:
        for event_handler in self.event_handlers:
            event_handler(event)

    @property
    def address(self):
        return f'{self.host}:{self.port}'

    async def _subscribe_market_data(self):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.MarketDataServiceStub(channel)

            request = bbgo_pb2.SubscribeRequest(subscriptions=[s.to_pb() for s in self.subscriptions])
            async for response in stub.Subscribe(request):
                event = MarketDataEvent.from_pb(response)
                self.fire_event_handlers(event)

    async def _subscribe_user_data(self, session: str):
        async with grpc.aio.insecure_channel(self.address) as channel:
            stub = bbgo_pb2_grpc.UserDataServiceStub(channel)

            request = bbgo_pb2.UserDataRequest(session=session)
            async for response in stub.Subscribe(request):
                event = UserDataEvent.from_pb(response)
                self.fire_event_handlers(event)

    def start(self):
        coroutines = [self._subscribe_market_data()]
        for session in self.sessions:
            coroutines.append(self._subscribe_user_data(session))

        group = asyncio.gather(*coroutines)
        loop = asyncio.get_event_loop()
        loop.run_until_complete(group)
        loop.close()
