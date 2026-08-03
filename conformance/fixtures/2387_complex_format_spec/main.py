cases = [
    (".2f", complex(1.5, 2.5)),
    (".3g", 3 + 4j),
    (".2e", 1 + 2j),
    (".1f", 1 - 2j),
    ("+.1f", 1 + 2j),
    (" ", 3 + 4j),
    (".2f", complex(0.0, 4.0)),
    ("F", 1 + 2j),
    ("E", 1 + 2j),
    ("n", 1 + 2j),
    ("#.0f", 1 + 2j),
    (".1f", complex(float("nan"), 2.0)),
    ("z.1f", complex(-0.0, 2.0)),
    (",.1f", complex(1234.5, 6789.1)),
    ("_.1f", complex(1234.5, 6789.1)),
    ("^20.1f", 1 + 2j),
    ("20.1f", 1 + 2j),
    ("<20.1f", 1 + 2j),
    ("*^20.1f", 1 + 2j),
    (">20", 3 + 4j),
    ("<20", 4j),
    ("+", complex(0.0, 4.0)),
    (",", complex(1234.5, 6789.1)),
    (".3", complex(1234.5, 6789.1)),
]
for spec, value in cases:
    print(repr(spec), repr(format(value, spec)))

# f-strings route through __format__ too.
z = 1.5 + 2.5j
print(f"{z:.2f}")
print(f"{z:>16.1f}")
print(f"{3 + 4j:.3g}")
print(f"{complex(0.0, 4.0)}")

errors = ["0.1f", "020.1f", "0<20.1f", "=20.1f", "d", "%", "s"]
for spec in errors:
    try:
        format(1 + 2j, spec)
    except ValueError as e:
        print(spec, "ValueError", e)
