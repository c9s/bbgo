from enum import Enum


class EventType(Enum):
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
