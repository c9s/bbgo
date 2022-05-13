from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

import bbgo_pb2

from ..enums import OrderType
from ..enums import SideType


@dataclass
class SubmitOrder:
    session: str
    exchange: str
    symbol: str
    side: SideType
    quantity: Decimal
    order_type: OrderType
    price: Decimal = None
    stop_price: Decimal = None
    client_order_id: str = None
    group_id: int = None

    def to_pb(self) -> bbgo_pb2.SubmitOrder:
        return bbgo_pb2.SubmitOrder(
            session=self.session,
            exchange=self.exchange,
            symbol=self.symbol,
            side=self.side.value,
            price=str(self.price or ""),
            quantity=str(self.quantity or ""),
            stop_price=str(self.stop_price or ""),
            order_type=self.order_type.value,
            client_order_id=self.client_order_id or "",
            group_id=self.group_id or 0,
        )
