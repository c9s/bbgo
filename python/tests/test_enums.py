from bbgo.enums import ChannelType


def test_channel_type_from_str():
    m = {
        'book': ChannelType.BOOK,
        'trade': ChannelType.TRADE,
        'ticker': ChannelType.TICKER,
        'kline': ChannelType.KLINE,
        'balance': ChannelType.BALANCE,
        'order': ChannelType.ORDER,
    }

    for k, v in m.items():
        assert ChannelType.from_str(k) == v
