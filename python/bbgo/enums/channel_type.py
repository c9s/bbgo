from enum import Enum


class ChannelType(Enum):
    BOOK = 'book'
    TRADE = 'trade'
    TICKER = 'ticker'
    USER = 'user'
    KLINE = 'kline'
