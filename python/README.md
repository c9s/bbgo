# pybbgo

## Installation

```sh
cd <path/to/bbgo/python>
pip install .
```

## Usage

### Stream

```python
from loguru import logger

from bbgo import Stream
from bbgo.data import Event
from bbgo.handlers import UpdateHandler


class LogBook(UpdateHandler):

    def handle(self, event: Event) -> None:
        logger.info(event)


host = '127.0.0.1'
port = 50051

stream = Stream(host, port)
stream.subscribe('max', 'book', 'BTCUSDT', 'full')
stream.subscribe('max', 'book', 'ETHUSDT', 'full')
stream.add_event_handler(LogBook())
stream.start()
```
