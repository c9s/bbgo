from __future__ import annotations

from enum import Enum


class ChannelType(Enum):
    BOOK = 0
    TRADE = 1
    TICKER = 2
    KLINE = 3
    BALANCE = 4
    ORDER = 5

    @classmethod
    def from_str(cls, s: str) -> ChannelType:
        return {t.name.lower(): t for t in cls}[s.lower()]
