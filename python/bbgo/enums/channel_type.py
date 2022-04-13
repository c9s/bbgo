from __future__ import annotations

from enum import Enum

import bbgo_pb2


class ChannelType(Enum):
    BOOK = 'book'
    TRADE = 'trade'
    TICKER = 'ticker'
    USER = 'user'
    KLINE = 'kline'

    @classmethod
    def from_pb(cls, obj) -> ChannelType:
        return {
            bbgo_pb2.Channel.BOOK: cls.BOOK,
            bbgo_pb2.Channel.TRADE: cls.TRADE,
            bbgo_pb2.Channel.TICKER: cls.TICKER,
            bbgo_pb2.Channel.USER: cls.USER,
            bbgo_pb2.Channel.KLINE: cls.KLINE,
        }[obj]

    def to_pb(self) -> bbgo_pb2.Channel:
        return {
            'book': bbgo_pb2.Channel.BOOK,
            'trade': bbgo_pb2.Channel.TRADE,
            'ticker': bbgo_pb2.Channel.TICKER,
            'user': bbgo_pb2.Channel.USER,
            'kline': bbgo_pb2.Channel.KLINE,
        }[self.value]
