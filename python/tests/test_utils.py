from bbgo.utils import parse_float
from bbgo.utils import parse_time


def test_parse_time():
    t = 1650610080000
    d = parse_time(t)

    assert d.timestamp() == t / 1000


def test_parse_float():
    assert parse_float(None) == 0
    assert parse_float("") == 0

    s = "3.14159265358979"
    f = 3.14159265358979
    assert parse_float(s) == f
