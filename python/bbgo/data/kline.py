from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime

import bbgo_pb2


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
            open=float(obj.open),
            high=float(obj.high),
            low=float(obj.low),
            close=float(obj.close),
            volume=float(obj.volume),
            quote_volume=float(obj.quote_volume),
            start_time=datetime.fromtimestamp(obj.start_time / 1000),
            end_time=datetime.fromtimestamp(obj.end_time / 1000),
            closed=obj.closed,
        )
