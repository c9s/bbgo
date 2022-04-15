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
