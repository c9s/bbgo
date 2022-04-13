import click
import grpc
from loguru import logger

import bbgo_pb2_grpc
from bbgo import MarketService
from bbgo.data import Subscription
from bbgo.enums import ChannelType
from bbgo.enums import DepthType


@click.command()
@click.option('--host', default='127.0.0.1')
@click.option('--port', default=50051)
def main(host, port):
    subscriptions = [
        Subscription('binance', ChannelType.BOOK, symbol='BTCUSDT', depth=DepthType.FULL),
    ]
    address = f'{host}:{port}'
    channel = grpc.insecure_channel(address)
    stub = bbgo_pb2_grpc.MarketDataServiceStub(channel)

    service = MarketService(stub)
    response_iter = service.subscribe(subscriptions)
    for response in response_iter:
        logger.info(response)


if __name__ == '__main__':
    main()
