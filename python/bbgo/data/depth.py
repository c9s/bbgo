from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from typing import List

import bbgo_pb2

from ..utils import parse_number


@dataclass
class Depth:
    exchange: str
    symbol: str
    asks: List[PriceVolume]
    bids: List[PriceVolume]

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Depth):
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            asks=[PriceVolume.from_pb(ask) for ask in obj.asks],
            bids=[PriceVolume.from_pb(bid) for bid in obj.bids],
        )


@dataclass
class PriceVolume:
    price: Decimal
    volume: Decimal

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.PriceVolume):
        return cls(
            price=parse_number(obj.price),
            volume=parse_number(obj.volume),
        )
