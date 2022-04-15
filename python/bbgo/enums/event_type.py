from __future__ import annotations

from enum import Enum


class EventType(Enum):
    UNKNOWN = 0
    SUBSCRIBED = 1
    UNSUBSCRIBED = 2
    SNAPSHOT = 3
    UPDATE = 4
    AUTHENTICATED = 5
    ERROR = 99

    @classmethod
    def from_str(cls, s: str) -> EventType:
        return {t.name.lower(): t for t in cls}[s.lower()]
