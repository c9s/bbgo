from ..data import Event
from ..enums import EventType
from .handler import Handler


class SnapshotHandler(Handler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.SNAPSHOT:
            return

        super(SnapshotHandler, self).__call__(event)
