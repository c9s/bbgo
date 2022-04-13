import click
from loguru import logger

import bbgo_pb2
from bbgo import Stream


@click.command()
@click.option('--host', default='127.0.0.1')
@click.option('--port', default=50051)
def main(host, port):
    subscriptions = [
        bbgo_pb2.Subscription(exchange='max', channel=bbgo_pb2.Channel.BOOK, symbol='BTCUSDT', depth='full'),
        bbgo_pb2.Subscription(exchange='binance', channel=bbgo_pb2.Channel.BOOK, symbol='BTCUSDT', depth='full'),
    ]

    def book_event_callback(event):
        logger.info(event)

    stream = Stream(host, port, subscriptions)
    stream.on_book_event(book_event_callback)

    stream.start()


if __name__ == '__main__':
    main()
