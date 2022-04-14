from __future__ import annotations

from enum import Enum

import bbgo_pb2


class EventType(Enum):
    UNKNOWN = 'unknown'
    ERROR = 'error'
    SUBSCRIBED = 'subscribed'
    UNSUBSCRIBED = 'unsubscribed'
    AUTHENTICATED = 'authenticated'
    SNAPSHOT = 'snapshot'
    UPDATE = 'update'

    @staticmethod
    def from_pb(obj: bbgo_pb2.Event) -> EventType:
        d = {
            bbgo_pb2.Event.UNKNOWN: EventType.UNKNOWN,
            bbgo_pb2.Event.ERROR: EventType.ERROR,
            bbgo_pb2.Event.SUBSCRIBED: EventType.SUBSCRIBED,
            bbgo_pb2.Event.UNSUBSCRIBED: EventType.UNSUBSCRIBED,
            bbgo_pb2.Event.AUTHENTICATED: EventType.AUTHENTICATED,
            bbgo_pb2.Event.SNAPSHOT: EventType.SNAPSHOT,
            bbgo_pb2.Event.UPDATE: EventType.UPDATE,
        }
        return d[obj]
