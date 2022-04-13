import click
from loguru import logger

from bbgo import Stream
from bbgo.data import Event
from bbgo.handlers import UpdateHandler


class LogBook(UpdateHandler):

    def handle(self, event: Event) -> None:
        logger.info(event)


@click.command()
@click.option('--host', default='127.0.0.1')
@click.option('--port', default=50051)
def main(host, port):
    stream = Stream(host, port)
    stream.subscribe('max', 'book', 'BTCUSDT', 'full')
    stream.subscribe('max', 'book', 'ETHUSDT', 'full')
    stream.add_event_handler(LogBook())
    stream.start()


if __name__ == '__main__':
    main()
