from ..data import MarketDataEvent
from ..enums import ChannelType
from ..enums import EventType
from .handler import Handler


class TickerHandler(Handler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.channel_type != ChannelType.TICKER:
            return

        super(TickerHandler, self).__call__(event)


class TickerSnapshotHandler(TickerHandler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.event_type != EventType.SNAPSHOT:
            return

        super(TickerSnapshotHandler, self).__call__(event)


class TickerUpdateHandler(TickerHandler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.event_type != EventType.UPDATE:
            return

        super(TickerUpdateHandler, self).__call__(event)
