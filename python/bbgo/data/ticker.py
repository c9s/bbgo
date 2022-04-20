from __future__ import annotations

from dataclasses import dataclass

import bbgo_pb2

from ..utils import parse_float


@dataclass
class Ticker:
    exchange: str
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: float

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.KLine) -> Ticker:
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            open=parse_float(obj.open),
            high=parse_float(obj.high),
            low=parse_float(obj.low),
            close=parse_float(obj.close),
            volume=parse_float(obj.volume),
        )
