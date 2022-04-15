from __future__ import annotations

from enum import Enum


class ChannelType(Enum):
    BOOK = 0
    TRADE = 1
    TICKER = 2
    KLINE = 3
    BALANCE = 4
    ORDER = 5

    @staticmethod
    def from_str(s: str) -> ChannelType:
        return {t.name.lower(): t for t in ChannelType}[s]
