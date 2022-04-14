from __future__ import annotations

from dataclasses import dataclass
from typing import List

import bbgo_pb2

from ..enums import ChannelType
from ..enums import EventType
from .balance import Balance
from .depth import Depth
from .order import Order
from .trade import Trade


class Event:
    pass


@dataclass
class UserDataEvent(Event):
    session: str
    exchange: str
    channel_type: ChannelType
    event_type: EventType
    balances: List[Balance] = None
    trades: List[Trade] = None
    orders: List[Order] = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.UserData) -> UserDataEvent:
        return cls(
            session=obj.session,
            exchange=obj.exchange,
            channel_type=ChannelType.from_pb(obj.channel),
            event_type=EventType.from_pb(obj.event),
            balances=[Balance.from_pb(balance) for balance in obj.balances],
            trades=[Trade.from_pb(trade) for trade in obj.trades],
            orders=[Order.from_pb(order) for order in obj.orders],
        )


@dataclass
class MarketDataEvent(Event):
    exchange: str
    symbol: str
    channel_type: ChannelType
    event_type: EventType
    depth: Depth = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.SubscribeResponse) -> MarketDataEvent:
        channel_type = ChannelType.from_pb(obj.channel)

        event = cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            channel_type=channel_type,
            event_type=EventType.from_pb(obj.event),
        )

        if channel_type == ChannelType.BOOK:
            event.depth = Depth.from_pb(obj.depth)

        return event
