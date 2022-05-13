from decimal import Decimal

from bbgo.utils import parse_number
from bbgo.utils import parse_time


def test_parse_time():
    t = 1650610080000
    d = parse_time(t)

    assert d.timestamp() == t / 1000


def test_parse_float():
    assert parse_number(None) == 0
    assert parse_number("") == 0

    s = "3.14159265358979"
    assert parse_number(s) == Decimal(s)
