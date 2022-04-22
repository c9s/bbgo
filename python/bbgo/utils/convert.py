from datetime import datetime
from typing import Union


def parse_float(s: Union[str, float]) -> float:
    if s is None:
        return 0

    if s == "":
        return 0

    return float(s)


def parse_time(t: Union[str, int]) -> datetime:
    if isinstance(t, str):
        t = int(t)

    return datetime.fromtimestamp(t / 1000)
