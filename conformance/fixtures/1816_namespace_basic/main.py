# A directory with no __init__.py is a PEP 420 namespace package: it imports as
# a bodyless module whose __file__ is None and whose __path__ is the single
# directory, and its submodules still execute and bind on it.
import geo

print("type:", type(geo).__name__)
print("name:", geo.__name__)
print("file:", geo.__file__)
print("path entries:", len(geo.__path__))
print("path basename:", geo.__path__[0].rsplit("/", 1)[-1])
print("has body attr:", hasattr(geo, "marker"))

import geo.shapes

print("shapes:", geo.shapes.area)

from geo import shapes as s2

print("via from:", s2.area)
