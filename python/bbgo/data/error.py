from __future__ import annotations

from dataclasses import dataclass

import bbgo_pb2


@dataclass
class ErrorMessage:
    code: int
    message: str

    @classmethod
    def from_pb(cls, obj: bbgo_pb2.Error) -> ErrorMessage:
        return cls(
            code=obj.error_code,
            message=obj.error_message,
        )
