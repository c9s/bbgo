# pybbgo

## Installation

```sh
cd <path/to/bbgo/python>
pip install .
```

## Usage

### Stream

```python
from bbgo import Stream
from bbgo import bbgo_pb2

subscriptions = [
    bbgo_pb2.Subscription(exchange='max', channel=bbgo_pb2.Channel.BOOK, symbol='btcusdt', depth=2),
    bbgo_pb2.Subscription(exchange='max', channel=bbgo_pb2.Channel.BOOK, symbol='ethusdt', depth=2),
    ...
]

stream = Stream(host, port, subscriptions)
stream.on_book_event(book_event_callback)
stream.on_ticker_event(ticker_event_callback)
...
stream.start()
```
