from dataclasses import dataclass
from datetime import datetime


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
