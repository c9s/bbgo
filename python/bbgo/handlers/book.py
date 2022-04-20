from ..data import Event
from ..enums import ChannelType
from ..enums import EventType
from .handler import Handler


class BookHandler(Handler):

    def __call__(self, event: Event) -> None:
        if event.channel_type != ChannelType.BOOK:
            return

        super(BookHandler, self).__call__(event)


class BookSnapshotHandler(BookHandler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.SNAPSHOT:
            return

        super(BookSnapshotHandler, self).__call__(event)


class BookUpdateHandler(BookHandler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.UPDATE:
            return

        super(BookUpdateHandler, self).__call__(event)
