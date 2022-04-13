from ..data import Event
from ..enums import EventType
from .handler import Handler


class UpdateHandler(Handler):

    def __call__(self, event: Event) -> None:
        if event.event_type != EventType.UPDATE:
            return

        super(UpdateHandler, self).__call__(event)
