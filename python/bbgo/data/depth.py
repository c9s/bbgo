from __future__ import annotations

from dataclasses import dataclass
import bbgo_pb2

from typing import List


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
    price: float
    volume: float

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.PriceVolume):
        return cls(
            price=float(obj.price),
            volume=float(obj.volume),
        )
