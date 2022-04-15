from __future__ import annotations

from enum import Enum


class OrderType(Enum):
    MARKET = 0
    LIMIT = 1
    STOP_MARKET = 2
    STOP_LIMIT = 3
    POST_ONLY = 4
    IOC_LIMIT = 5
