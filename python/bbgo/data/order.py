from dataclasses import dataclass
from datetime import datetime

from ..enums import OrderType
from ..enums import SideType


@dataclass
class Order:
    order_id: str
    side: SideType
    order_type: OrderType
    price: float
    stop_price: float
    average_price: float
    status: str
    market: str
    created_at: datetime
    volume: float
    remaining_volume: float
    executed_volume: float
    trade_count: int
    client_order_id: str
    group_id: str
