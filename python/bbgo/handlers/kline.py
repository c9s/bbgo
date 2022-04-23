from ..data import MarketDataEvent
from ..enums import ChannelType
from ..enums import EventType
from .handler import Handler


class KLineHandler(Handler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.channel_type != ChannelType.KLINE:
            return

        super(KLineHandler, self).__call__(event)


class KLineSnapshotHandler(KLineHandler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.event_type != EventType.SNAPSHOT:
            return

        super(KLineSnapshotHandler, self).__call__(event)


class KLineUpdateHandler(KLineHandler):

    def __call__(self, event: MarketDataEvent) -> None:
        if event.event_type != EventType.UPDATE:
            return

        super(KLineUpdateHandler, self).__call__(event)
