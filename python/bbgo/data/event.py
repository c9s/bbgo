from __future__ import annotations

from dataclasses import dataclass

import bbgo_pb2

from ..enums import EventType, ChannelType
from . import Depth


@dataclass
class Event:
    exchange: str
    symbol: str
    channel_type: ChannelType
    event_type: EventType
    depth: Depth = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.SubscribeResponse) -> Event:
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
