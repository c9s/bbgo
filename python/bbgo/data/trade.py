from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime

import bbgo_pb2

from ..enums import SideType


@dataclass
class Trade:
    session: str
    exchange: str
    symbol: str
    id: str
    price: float
    quantity: float
    created_at: datetime
    side: SideType
    fee_currency: str
    fee: float
    maker: bool

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Trade) -> Trade:
        return cls(
            session=obj.session,
            exchange=obj.exchange,
            symbol=obj.symbol,
            id=obj.id,
            price=float(obj.price),
            quantity=float(obj.quantity),
            created_at=datetime.fromtimestamp(obj.created_at / 1000),
            side=SideType(obj.side),
            fee_currency=obj.fee_currency,
            fee=float(obj.fee),
            maker=obj.maker,
        )
