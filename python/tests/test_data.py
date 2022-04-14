from datetime import datetime

import bbgo_pb2
from bbgo.data import KLine, ErrorMessage


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
    assert kline.open == float(open)
    assert kline.high == float(high)
    assert kline.low == float(low)
    assert kline.close == float(close)
    assert kline.volume == float(volume)
    assert kline.quote_volume == float(quote_volume)
    assert kline.start_time == datetime.fromtimestamp(start_time / 1000)
    assert kline.end_time == datetime.fromtimestamp(end_time / 1000)
    assert closed == closed


def test_order_from_pb():
    error_code = 123
    error_message = "error message 123"

    error_pb = bbgo_pb2.Error(error_code=error_code, error_message=error_message)
    error = ErrorMessage.from_pb(error_pb)

    assert error.code == error_code
    assert error.message == error_message
