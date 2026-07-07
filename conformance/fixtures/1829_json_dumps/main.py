# json is a pure-Python module with a C accelerator in CPython, provided in Go
# behind the json import. This exercises the encoder: the scalar spellings, the
# escaping that ensure_ascii toggles, nested containers, indent and separators,
# sort_keys, key coercion, and the error cases.
import json

print(json.dumps(None), json.dumps(True), json.dumps(False))
print(json.dumps(42), json.dumps(-7), json.dumps(10 ** 30))
print(json.dumps(1.5), json.dumps(2.0), json.dumps(1e20))
print(json.dumps("hi"))
print(json.dumps('a"b\\c\n\td'))
print(json.dumps([1, 2, 3]))
print(json.dumps((1, "two", None)))
print(json.dumps({"b": 1, "a": 2}))
print(json.dumps({"nested": {"x": [1, {"y": 2}]}}))

print(json.dumps("café"))
print(json.dumps("café", ensure_ascii=False))
print(json.dumps("emoji \U0001f600 here"))

print(json.dumps({"b": 1, "a": 2}, sort_keys=True))
print(json.dumps([1, 2], indent=2))
print(json.dumps({"a": 1, "b": [2, 3]}, indent=2))
print(json.dumps({"a": 1, "b": 2}, indent=4, sort_keys=True))
print(json.dumps([1, 2, 3], separators=(",", ":")))
print(json.dumps({"a": 1, "b": 2}, separators=(";", "=")))

print(json.dumps({1: "int", 2.5: "float", True: "bool", None: "none"}))
print(json.dumps({"keep": 1, (1, 2): "drop"}, skipkeys=True))

print(json.dumps(float("inf")), json.dumps(float("nan")))


def show(fn):
    try:
        fn()
    except (TypeError, ValueError) as e:
        print(type(e).__name__ + ":", e)


show(lambda: json.dumps({1, 2, 3}))
show(lambda: json.dumps(object()))
show(lambda: json.dumps({(1, 2): "x"}))
show(lambda: json.dumps(float("nan"), allow_nan=False))


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y


def encode_point(o):
    if isinstance(o, Point):
        return {"x": o.x, "y": o.y}
    raise TypeError(f"Object of type {type(o).__name__} is not JSON serializable")


print(json.dumps(Point(1, 2), default=encode_point))
