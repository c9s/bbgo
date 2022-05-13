from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import bbgo_pb2

from ..utils import parse_number
from ..utils import parse_time


@dataclass
class KLine:
    exchange: str
    symbol: str
    open: Decimal
    high: Decimal
    low: Decimal
    close: Decimal
    volume: Decimal
    session: str = None
    start_time: datetime = None
    end_time: datetime = None
    quote_volume: Decimal = None
    closed: bool = None

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.KLine) -> KLine:
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            open=parse_number(obj.open),
            high=parse_number(obj.high),
            low=parse_number(obj.low),
            close=parse_number(obj.close),
            volume=parse_number(obj.volume),
            quote_volume=parse_number(obj.quote_volume),
            start_time=parse_time(obj.start_time),
            end_time=parse_time(obj.end_time),
            closed=obj.closed,
        )
