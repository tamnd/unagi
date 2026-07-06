from shapes import area, side
from shapes import Square as Sq

print(area(2, 3), side)
print(Sq(4).area())

try:
    from shapes import nothere
except ImportError as e:
    print(type(e).__name__, e)

try:
    import missing
except ModuleNotFoundError as e:
    print(type(e).__name__, e)

try:
    import also_missing as m
except ImportError as e:
    print(type(e).__name__, e)

import shapes
print(shapes.side is side)
shapes.side = 7
print(shapes.side, side)
