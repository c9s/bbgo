import click

from bbgo import MarketService


@click.command()
@click.option('--host', default='127.0.0.1')
@click.option('--port', default=50051)
def main(host, port):
    service = MarketService(host, port)

    klines, error = service.query_klines(exchange='binance', symbol='BTCUSDT', interval='1m', limit=10)

    for kline in klines:
        print(kline)

    print(error)


if __name__ == '__main__':
    main()
