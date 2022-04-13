from enum import Enum


class OrderType(Enum):
    MARKET = 'market'
    LIMIT = 'limit'
    STOP_MARKET = 'stop_market'
    STOP_LIMIT = 'stop_limit'
    POST_ONLY = 'post_only'
    IOC_LIMIT = 'ioc_limit'
