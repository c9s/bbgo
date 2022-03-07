from concurrent import futures

import grpc
import pytest

from bbgo import BBGO
from bbgo import bbgo_pb2
from bbgo import bbgo_pb2_grpc
from tests.servicer import TestServicer


@pytest.fixture
def grpc_address(host='[::]', port=50051):
    return f'{host}:{port}'


@pytest.fixture
def bbgo(host='[::]', port=50051):
    return BBGO(host, port)


@pytest.fixture
def grpc_channel(grpc_address):
    return grpc.insecure_channel(grpc_address)


@pytest.fixture
def grpc_server(grpc_address, max_workers=1):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers))
    servicer = TestServicer()
    bbgo_pb2_grpc.add_BBGOServicer_to_server(servicer, server)
    server.add_insecure_port(grpc_address)
    server.start()
    yield server
    server.stop(grace=None)


def test_submit_order(bbgo, grpc_server):
    exchange = 'max'
    symbol = 'BTCUSDT'
    side = bbgo_pb2.Side.BUY
    quantity = 0.01
    order_type = bbgo_pb2.OrderType.LIMIT

    response = bbgo.submit_order(
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
