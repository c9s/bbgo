from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

import bbgo_pb2

from ..utils import parse_number


@dataclass
class Ticker:
    exchange: str
    symbol: str
    open: Decimal
    high: Decimal
    low: Decimal
    close: Decimal
    volume: Decimal

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.KLine) -> Ticker:
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            open=parse_number(obj.open),
            high=parse_number(obj.high),
            low=parse_number(obj.low),
            close=parse_number(obj.close),
            volume=parse_number(obj.volume),
        )
