from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import bbgo_pb2

from ..enums import SideType
from ..utils import parse_number
from ..utils import parse_time


@dataclass
class Trade:
    session: str
    exchange: str
    symbol: str
    trade_id: str
    price: Decimal
    quantity: Decimal
    created_at: datetime
    side: SideType
    fee_currency: str
    fee: Decimal
    maker: bool

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Trade) -> Trade:
        return cls(
            session=obj.session,
            exchange=obj.exchange,
            symbol=obj.symbol,
            trade_id=obj.id,
            price=parse_number(obj.price),
            quantity=parse_number(obj.quantity),
            created_at=parse_time(obj.created_at),
            side=SideType(obj.side),
            fee_currency=obj.fee_currency,
            fee=parse_number(obj.fee),
            maker=obj.maker,
        )
