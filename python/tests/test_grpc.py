from concurrent import futures

import bbgo_pb2
import bbgo_pb2_grpc
import grpc
import pytest

from bbgo import MarketService
from bbgo import TradingService
from tests.servicer import TestTradingServicer


@pytest.fixture
def address(host='[::]', port=50051):
    return f'{host}:{port}'


@pytest.fixture
def channel(address):
    return grpc.insecure_channel(address)


@pytest.fixture
def trading_service(channel):
    trading_service_stub = bbgo_pb2_grpc.TradingServiceStub(channel)
    return TradingService(trading_service_stub)


@pytest.fixture
def market_service(channel):
    market_service_stub = bbgo_pb2_grpc.MarketDataServiceStub(channel)
    return MarketService(market_service_stub)


@pytest.fixture
def test_trading_servicer(address, max_workers=1):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers))
    servicer = TestTradingServicer()
    bbgo_pb2_grpc.add_TradingServiceServicer_to_server(servicer, server)
    server.add_insecure_port(address)
    server.start()
    yield server
    server.stop(grace=None)


def test_submit_order(trading_service, test_trading_servicer):
    exchange = 'max'
    symbol = 'BTCUSDT'
    side = bbgo_pb2.Side.BUY
    quantity = 0.01
    order_type = bbgo_pb2.OrderType.LIMIT

    response = trading_service.submit_order(
        exchange=exchange,
        symbol=symbol,
        side=side,
        quantity=quantity,
        order_type=order_type,
    )

    order = response.order

    assert order.exchange == exchange
    assert order.symbol == symbol
    assert order.side == side
    assert order.quantity == quantity
