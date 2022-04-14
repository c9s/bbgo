from __future__ import annotations

from enum import Enum

import bbgo_pb2


class OrderType(Enum):
    MARKET = 'market'
    LIMIT = 'limit'
    STOP_MARKET = 'stop_market'
    STOP_LIMIT = 'stop_limit'
    POST_ONLY = 'post_only'
    IOC_LIMIT = 'ioc_limit'

    @staticmethod
    def from_pb(obj: bbgo_pb2.OrderType) -> OrderType:
        return {
            bbgo_pb2.OrderType.MARKET: OrderType.MARKET,
            bbgo_pb2.OrderType.LIMIT: OrderType.LIMIT,
            bbgo_pb2.OrderType.STOP_MARKET: OrderType.STOP_MARKET,
            bbgo_pb2.OrderType.STOP_LIMIT: OrderType.STOP_LIMIT,
            bbgo_pb2.OrderType.POST_ONLY: OrderType.POST_ONLY,
            bbgo_pb2.OrderType.IOC_LIMIT: OrderType.IOC_LIMIT,
        }[obj]
