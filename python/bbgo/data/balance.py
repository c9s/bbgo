from dataclasses import dataclass


@dataclass
class Balance:
    exchange: str
    currency: str
    available: float
    locked: float
