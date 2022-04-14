from __future__ import annotations

from dataclasses import dataclass

import bbgo_pb2


@dataclass
class Balance:
    exchange: str
    currency: str
    available: float
    locked: float
    borrowed: str

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Balance) -> Balance:
        return cls(
            exchange=obj.exchange,
            currency=obj.currency,
            available=float(obj.available),
            locked=float(obj.locked),
            borrowed=obj.borrowed,
        )

    def total(self) -> float:
        return self.available + self.locked
