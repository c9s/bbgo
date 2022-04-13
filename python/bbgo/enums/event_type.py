from __future__ import annotations

from enum import Enum

# enum Event {
#   UNKNOWN = 0;
#   SUBSCRIBED = 1;
#   UNSUBSCRIBED = 2;
#   SNAPSHOT = 3;
#   UPDATE = 4;
#   AUTHENTICATED = 5;
#   ORDER_SNAPSHOT = 6;
#   ORDER_UPDATE = 7;
#   TRADE_SNAPSHOT = 8;
#   TRADE_UPDATE = 9;
#   ACCOUNT_SNAPSHOT = 10;
#   ACCOUNT_UPDATE = 11;
#   ERROR = 99;
# }
import bbgo_pb2


class EventType(Enum):
    UNKNOWN = 'unknown'
    ERROR = 'error'
    SUBSCRIBED = 'subscribed'
    UNSUBSCRIBED = 'unsubscribed'
    AUTHENTICATED = 'authenticated'
    SNAPSHOT = 'snapshot'
    UPDATE = 'update'
    ORDER_SNAPSHOT = 'order_snapshot'
    ORDER_UPDATE = 'order_update'
    TRADE_SNAPSHOT = 'trade_snapshot'
    TRADE_UPDATE = 'trade_update'
    ACCOUNT_SNAPSHOT = 'account_snapshot'
    ACCOUNT_UPDATE = 'account_update'

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
            bbgo_pb2.Event.ORDER_SNAPSHOT: cls.ORDER_SNAPSHOT,
            bbgo_pb2.Event.ORDER_UPDATE: cls.ORDER_UPDATE,
            bbgo_pb2.Event.TRADE_SNAPSHOT: cls.TRADE_SNAPSHOT,
            bbgo_pb2.Event.TRADE_UPDATE: cls.TRADE_UPDATE,
            bbgo_pb2.Event.ACCOUNT_SNAPSHOT: cls.ACCOUNT_SNAPSHOT,
            bbgo_pb2.Event.ACCOUNT_UPDATE: cls.ACCOUNT_UPDATE,
        }
        return d[obj]
