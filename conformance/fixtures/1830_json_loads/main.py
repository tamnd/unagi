# The json decoder, provided in Go behind the json import. This exercises
# json.loads: the scalar spellings, exact integers, the JavaScript constants,
# string escapes and surrogate pairs, nested containers, duplicate keys, and the
# JSONDecodeError surface with its msg, pos, lineno, and colno.
import json
from json import JSONDecodeError

print(json.loads("null"), json.loads("true"), json.loads("false"))
print(json.loads("42"), json.loads("-7"), json.loads("100000000000000000000000000"))
print(json.loads("1.5"), json.loads("2.0"), json.loads("1e3"), json.loads("-0.25"))
print(json.loads('"hi"'))
print(json.loads(r'"a\"b\\c\n\td"'))
print(json.loads('"caf\\u00e9"'))
print(json.loads('"\\ud834\\udd1e"'))
print(json.loads("[1, 2, 3]"))
print(json.loads('[1, "two", null, true]'))
print(json.loads('{"a": 1, "b": [2, 3]}'))
print(json.loads('{"nested": {"x": [1, {"y": 2}]}}'))
print(json.loads("  \t\n [ 1 ,2, 3 ] "))

print(json.loads("NaN"), json.loads("Infinity"), json.loads("-Infinity"))

vals = json.loads('[1, 1.5, 1e3, true, false, null, "s"]')
print([type(x).__name__ for x in vals])

print(json.loads('{"a": 1, "a": 2}'))

print(isinstance(json.loads, type(json.loads)))
print(issubclass(json.JSONDecodeError, ValueError))


def show(s):
    try:
        json.loads(s)
    except JSONDecodeError as e:
        print(type(e).__name__, "|", e.msg, "|", e.pos, e.lineno, e.colno, "|", str(e))


show("")
show("   ")
show("nul")
show("{")
show("[1,2")
show("[1,2,]")
show('{"a":1,}')
show('{"a" 1}')
show('{"a":1 "b":2}')
show("{1:2}")
show('"abc')
show('"a\tb"')
show(r'"a\x"')
show(r'"a\u12"')
show("1 2")
show("01")
show('{\n  "a": bad\n}')


def caught_by_valueerror(s):
    try:
        json.loads(s)
    except ValueError as e:
        return type(e).__name__


print(caught_by_valueerror(""))
