from __future__ import annotations

from dataclasses import dataclass

import bbgo_pb2

from ..enums import ChannelType
from ..enums import DepthType


@dataclass
class Subscription:
    exchange: str
    channel: ChannelType
    symbol: str
    depth: DepthType = None
    interval: str = None

    def to_pb(self) -> bbgo_pb2.Subscription:
        subscription_pb = bbgo_pb2.Subscription(
            exchange=self.exchange,
            channel=self.channel.value,
            symbol=self.symbol,
        )

        if self.depth is not None:
            subscription_pb.depth = self.depth.value

        if self.interval is not None:
            subscription_pb.interval = self.interval

        return subscription_pb
