import click
import grpc
import bbgo_pb2
import bbgo_pb2_grpc
from bbgo import MarketService


@click.command()
@click.option('--host', default='127.0.0.1')
@click.option('--port', default=50051)
def main(host, port):
    address = f'{host}:{port}'
    channel = grpc.insecure_channel(address)

    stub = bbgo_pb2_grpc.MarketDataServiceStub(channel)
    service = MarketService(stub)

    klines, error = service.query_klines(exchange='binance', symbol='BTCUSDT', interval='1m', limit=10)

    for kline in klines:
        print(kline)

    print(error)


if __name__ == '__main__':
    main()
