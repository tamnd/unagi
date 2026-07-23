# Class-body variable annotations populate __annotations__, evaluated eagerly.
# Under PEP 649 the mapping lives behind the __annotations__ descriptor, not in
# the class dict, so `'__annotations__' in C.__dict__` stays false in 3.14.

class C:
    x: int
    y: str = "hi"
    z: "Forward"

print("ann", C.__annotations__)
print("value bound", C.y)
print("in dict", "__annotations__" in C.__dict__)
print("memoized", C.__annotations__ is C.__annotations__)

# A bare annotation records the name but binds no class attribute.
try:
    C.x
    print("x bound")
except AttributeError:
    print("x not bound")

# A class that declares no annotations reads back an empty dict.
class Empty:
    pass

print("empty", Empty.__annotations__)
print("empty is dict", isinstance(Empty.__annotations__, dict))
print("empty in dict", "__annotations__" in Empty.__dict__)

# The getset descriptor annotationlib binds off type.__dict__.
d = type.__dict__["__annotations__"]
print("descr", repr(d))
get = d.__get__
print("get C", get(C))
print("get Empty", get(Empty))
try:
    get(int)
    print("int has annotations")
except AttributeError:
    print("static type AttributeError")

# annotationlib imports now that the descriptor exists.
import annotationlib
print("annotationlib", annotationlib.__name__)
