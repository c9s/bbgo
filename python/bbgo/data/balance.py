from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

import bbgo_pb2

from ..utils import parse_number


@dataclass
class Balance:
    exchange: str
    currency: str
    available: Decimal
    locked: Decimal
    borrowed: Decimal

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Balance) -> Balance:
        return cls(
            exchange=obj.exchange,
            currency=obj.currency,
            available=parse_number(obj.available),
            locked=parse_number(obj.locked),
            borrowed=parse_number(obj.borrowed),
        )

    def total(self) -> Decimal:
        return self.available + self.locked
