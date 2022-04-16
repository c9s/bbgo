from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime

import bbgo_pb2

from ..utils import parse_float


@dataclass
class KLine:
    exchange: str
    symbol: str
    open: float
    high: float
    low: float
    close: float
    volume: float
    session: str = None
    start_time: datetime = None
    end_time: datetime = None
    quote_volume: float = None
    closed: bool = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.KLine) -> KLine:
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            open=parse_float(obj.open),
            high=parse_float(obj.high),
            low=parse_float(obj.low),
            close=parse_float(obj.close),
            volume=parse_float(obj.volume),
            quote_volume=parse_float(obj.quote_volume),
            start_time=datetime.fromtimestamp(obj.start_time / 1000),
            end_time=datetime.fromtimestamp(obj.end_time / 1000),
            closed=obj.closed,
        )
