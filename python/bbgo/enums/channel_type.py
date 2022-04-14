from __future__ import annotations

from enum import Enum

import bbgo_pb2


class ChannelType(Enum):
    BOOK = 'book'
    TRADE = 'trade'
    TICKER = 'ticker'
    KLINE = 'kline'

    @staticmethod
    def from_pb(obj) -> ChannelType:
        return {
            bbgo_pb2.Channel.BOOK: ChannelType.BOOK,
            bbgo_pb2.Channel.TRADE: ChannelType.TRADE,
            bbgo_pb2.Channel.TICKER: ChannelType.TICKER,
            bbgo_pb2.Channel.KLINE: ChannelType.KLINE,
        }[obj]

    def to_pb(self) -> bbgo_pb2.Channel:
        return {
            'book': bbgo_pb2.Channel.BOOK,
            'trade': bbgo_pb2.Channel.TRADE,
            'ticker': bbgo_pb2.Channel.TICKER,
            'kline': bbgo_pb2.Channel.KLINE,
        }[self.value]
