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

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Event) -> EventType:
        d = {
            bbgo_pb2.Event.UNKNOWN: cls.UNKNOWN,
            bbgo_pb2.Event.ERROR: cls.ERROR,
            bbgo_pb2.Event.SUBSCRIBED: cls.SUBSCRIBED,
            bbgo_pb2.Event.UNSUBSCRIBED: cls.UNSUBSCRIBED,
            bbgo_pb2.Event.AUTHENTICATED: cls.AUTHENTICATED,
            bbgo_pb2.Event.SNAPSHOT: cls.SNAPSHOT,
            bbgo_pb2.Event.UPDATE: cls.UPDATE,
        }
        return d[obj]
