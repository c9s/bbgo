from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import List

import bbgo_pb2

from ..enums import ChannelType
from ..enums import EventType
from ..utils import parse_time
from .balance import Balance
from .depth import Depth
from .error import ErrorMessage
from .kline import KLine
from .order import Order
from .ticker import Ticker
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
            channel_type=ChannelType(obj.channel),
            event_type=EventType(obj.event),
            balances=[Balance.from_pb(balance) for balance in obj.balances],
            trades=[Trade.from_pb(trade) for trade in obj.trades],
            orders=[Order.from_pb(order) for order in obj.orders],
        )


@dataclass
class MarketDataEvent(Event):
    session: str
    exchange: str
    symbol: str
    channel_type: ChannelType
    event_type: EventType
    subscribed_at: datetime
    error: ErrorMessage
    depth: Depth = None
    kline: KLine = None
    ticker: Ticker = None
    trades: List[Trade] = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.MarketData) -> MarketDataEvent:
        channel_type = ChannelType(obj.channel)

        event = cls(
            session=obj.session,
            exchange=obj.exchange,
            symbol=obj.symbol,
            channel_type=channel_type,
            event_type=EventType(obj.event),
            subscribed_at=parse_time(obj.subscribed_at),
            error=ErrorMessage.from_pb(obj.error),
        )

        if channel_type == ChannelType.BOOK:
            event.depth = Depth.from_pb(obj.depth)

        if channel_type == ChannelType.KLINE:
            event.kline = KLine.from_pb(obj.kline)

        if channel_type == ChannelType.TICKER:
            event.ticker = Ticker.from_pb(obj.ticker)

        if channel_type == ChannelType.TRADE:
            event.trades = [Trade.from_pb(trade) for trade in obj.trades]

        return event
