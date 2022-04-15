from __future__ import annotations

from enum import Enum


class SideType(Enum):
    BUY = 0
    SELL = 1

    @classmethod
    def from_str(cls, s: str) -> SideType:
        return {t.name.lower(): t for t in cls}[s.lower()]
