import cmath

# constants
print(cmath.pi, cmath.e, cmath.tau)
print(cmath.inf, cmath.nan)
print(cmath.infj, cmath.nanj)
print(type(cmath.pi).__name__, type(cmath.infj).__name__)

# sqrt and exp
print(cmath.sqrt(-1))
print(cmath.sqrt(complex(0, 2)))
print(cmath.exp(complex(0, 0)))

# phase, polar, rect
print(cmath.phase(complex(0, 1)))
print(cmath.phase(complex(-1, 0)))
print(cmath.polar(complex(0, 1)))
print(cmath.rect(1, 0))
print(cmath.rect(3, 0))

# log
print(cmath.log(complex(1, 0)))
print(cmath.log(100, 10))
print(cmath.log10(complex(100, 0)))

# predicates
print(cmath.isfinite(complex(1, 2)))
print(cmath.isinf(cmath.infj))
print(cmath.isnan(cmath.nanj))
print(cmath.isfinite(cmath.inf))

# isclose
print(cmath.isclose(complex(1, 1), complex(1, 1)))
print(cmath.isclose(complex(1, 0), complex(1, 1e-12)))
print(cmath.isclose(complex(1, 0), complex(2, 0)))
print(cmath.isclose(complex(1, 0), complex(1.0000001, 0), rel_tol=1e-3))

# errors
try:
    cmath.log(0)
except ValueError as e:
    print("ValueError", e)
try:
    cmath.sqrt("a")
except TypeError as e:
    print("TypeError", e)
