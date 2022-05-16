from decimal import Decimal

import bbgo_pb2
from bbgo.data import Balance
from bbgo.data import ErrorMessage
from bbgo.data import KLine
from bbgo.utils import parse_time


def test_balance_from_pb():
    exchange = 'max'
    currency = 'BTCUSDT'
    available = '3.1415926'
    locked = '2.7182818'
    borrowed = '0.1234567'

    balance_pb = bbgo_pb2.Balance(
        exchange=exchange,
        currency=currency,
        available=available,
        locked=locked,
        borrowed=borrowed,
    )

    balance = Balance.from_pb(balance_pb)

    assert balance.exchange == exchange
    assert balance.currency == currency
    assert balance.available == Decimal(available)
    assert balance.locked == Decimal(locked)
    assert balance.borrowed == Decimal(borrowed)


def test_kline_from_pb():
    exchange = "binance"
    symbol = "BTCUSDT"
    open = "39919.31"
    high = "39919.32"
    low = "39919.31"
    close = "39919.31"
    volume = "0.27697"
    quote_volume = "11056.4530226"
    start_time = 1649833260000
    end_time = 1649833319999
    closed = True

    kline_pb = bbgo_pb2.KLine(exchange=exchange,
                              symbol=symbol,
                              open=open,
                              high=high,
                              low=low,
                              close=close,
                              volume=volume,
                              quote_volume=quote_volume,
                              start_time=start_time,
                              end_time=end_time,
                              closed=closed)

    kline = KLine.from_pb(kline_pb)

    assert kline.exchange == exchange
    assert kline.symbol == symbol
    assert kline.open == Decimal(open)
    assert kline.high == Decimal(high)
    assert kline.low == Decimal(low)
    assert kline.close == Decimal(close)
    assert kline.volume == Decimal(volume)
    assert kline.quote_volume == Decimal(quote_volume)
    assert kline.start_time == parse_time(start_time)
    assert kline.end_time == parse_time(end_time)
    assert closed == closed


def test_order_from_pb():
    error_code = 123
    error_message = "error message 123"

    error_pb = bbgo_pb2.Error(error_code=error_code, error_message=error_message)
    error = ErrorMessage.from_pb(error_pb)

    assert error.code == error_code
    assert error.message == error_message
