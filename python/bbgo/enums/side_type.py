from __future__ import annotations

from enum import Enum

import bbgo_pb2


class SideType(Enum):
    BUY = 'buy'
    SELL = 'sell'

    @staticmethod
    def from_pb(obj: bbgo_pb2.Side) -> SideType:
        return {
            bbgo_pb2.Side.BUY: SideType.BUY,
            bbgo_pb2.Side.SELL: SideType.SELL,
        }[obj]
