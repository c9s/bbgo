from ..data import Event
from ..enums import ChannelType
from ..enums import EventType
from .handler import Handler


class TradeHandler(Handler):

    def __call__(self, event: Event) -> None:
        if event.channel_type != ChannelType.TRADE:
            return

        super(TradeHandler, self).__call__(event)


class TradeSnapshotHandler(TradeHandler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.SNAPSHOT:
            return

        super(TradeSnapshotHandler, self).__call__(event)


class TradeUpdateHandler(TradeHandler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.UPDATE:
            return

        super(TradeUpdateHandler, self).__call__(event)
