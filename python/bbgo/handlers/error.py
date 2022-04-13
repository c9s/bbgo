from ..data import Event
from ..enums import EventType
from .handler import Handler


class ErrorHandler(Handler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.ERROR:
            return

        super(ErrorHandler, self).__call__(event)
