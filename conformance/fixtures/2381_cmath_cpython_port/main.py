import cmath

atoms = [float('-inf'), -0.0, 0.0, float('inf'), float('nan')]
fns = ["acos", "acosh", "asin", "asinh", "atan", "atanh", "cos", "cosh",
       "exp", "sin", "sinh", "sqrt", "tan", "tanh"]

# Every C99 inf/nan/signed-zero combination for each complex function. These
# drive the special-value tables ported cell for cell from CPython's
# Modules/cmathmodule.c, so the whole grid must match the oracle exactly.
for name in fns:
    f = getattr(cmath, name)
    for a in atoms:
        for b in atoms:
            z = complex(a, b)
            try:
                print(name, repr(z), "->", repr(f(z)))
            except ValueError as e:
                print(name, repr(z), "| ValueError", e)

# A finite large real part overflows exp/cosh/sinh to OverflowError rather
# than silently returning an infinity.
for name in ["exp", "cosh", "sinh"]:
    f = getattr(cmath, name)
    try:
        print(name, "big ->", repr(f(complex(1e4, 1.0))))
    except OverflowError as e:
        print(name, "big | OverflowError", e)

# atan and atanh sign and domain behaviour.
print("atan", repr(cmath.atan(complex(0.0, 2.0))))
print("atan2", repr(cmath.atan(complex(2.0, 0.5))))
print("atanh", repr(cmath.atanh(complex(2.0, 0.0))))

# tanh saturates for a large real part, sqrt handles finite and subnormal
# magnitudes through the scaling path.
print("tanh-sat", repr(cmath.tanh(complex(1e4, 1.0))))
print("sqrt-fin", repr(cmath.sqrt(complex(-4.0, 0.0))))
print("sqrt-tiny", repr(cmath.sqrt(complex(1e-300, 1e-300))))

# Module-level helpers and error paths.
print("log", repr(cmath.log(complex(3, 4))))
print("log-base", repr(cmath.log(complex(8, 0), 2)))
print("log10", repr(cmath.log10(complex(-100, 0))))
print("phase", repr(cmath.phase(complex(-1, 0))))
print("polar", repr(cmath.polar(complex(1, 1))))
print("rect", repr(cmath.rect(2.0, cmath.pi / 3)))
print("isclose", cmath.isclose(complex(1, 1), complex(1, 1.0000000001)))
print("isclose-far", cmath.isclose(complex(1, 0), complex(2, 0)))
print("isnan", cmath.isnan(complex(float('nan'), 0)))
print("isinf", cmath.isinf(complex(0, float('inf'))))
print("isfinite", cmath.isfinite(complex(1, 2)))
print("pi", cmath.pi, "e", cmath.e, "tau", cmath.tau)
print("inf", cmath.inf, "nan-is-nan", cmath.isnan(cmath.nan))
print("infj", cmath.infj, "nanj-is-nan", cmath.isnan(cmath.nanj.imag))
try:
    cmath.log(complex(0, 0))
except ValueError as e:
    print("log0 | ValueError", e)
