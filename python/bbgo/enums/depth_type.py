from __future__ import annotations

from enum import Enum


#  string depth = 4; // depth is for book, valid values are full, medium, 1, 5 and 20
class DepthType(Enum):
    FULL = 'full'
    MEDIUM = 'medium'
    DEPTH_1 = '1'
    DEPTH_5 = '5'
    DEPTH_20 = '20'
