from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

import bbgo_pb2

from ..enums import OrderType
from ..enums import SideType
from ..utils import parse_number
from ..utils import parse_time


@dataclass
class Order:
    exchange: str
    symbol: str
    order_id: str
    side: SideType
    order_type: OrderType
    price: Decimal
    stop_price: Decimal
    status: str
    quantity: Decimal
    executed_quantity: Decimal
    client_order_id: str
    group_id: int
    created_at: datetime

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Order) -> Order:
        return cls(
            exchange=obj.exchange,
            symbol=obj.symbol,
            order_id=obj.id,
            side=SideType(obj.side),
            order_type=OrderType(obj.order_type),
            price=parse_number(obj.price),
            stop_price=parse_number(obj.stop_price),
            status=obj.status,
            quantity=parse_number(obj.quantity),
            executed_quantity=parse_number(obj.executed_quantity),
            client_order_id=obj.client_order_id,
            group_id=obj.group_id,
            created_at=parse_time(obj.created_at),
        )
