# math is a C module in CPython, provided in Go behind the same import name.
# The transcendental results are wrapped in round(_, 12) so both interpreters
# print the same decimal; the exact-valued results and the integer routines go
# straight to stdout.
import math

print("pi:", round(math.pi, 12))
print("e:", round(math.e, 12))
print("tau:", round(math.tau, 12))
print("inf:", math.inf, "nan:", math.nan)

print("sqrt4:", math.sqrt(4))
print("sqrt2:", round(math.sqrt(2), 12))
print("exp0:", math.exp(0))
print("log_e:", round(math.log(math.e), 12))
print("log_8_2:", round(math.log(8, 2), 12))
print("log2_8:", math.log2(8))
print("log10_1000:", math.log10(1000))

print("sin0:", math.sin(0), "cos0:", math.cos(0))
print("sin_pi6:", round(math.sin(math.pi / 6), 12))
print("atan2:", round(math.atan2(1, 1), 12))

print("floor:", math.floor(3.7), math.floor(-3.2))
print("ceil:", math.ceil(3.2), math.ceil(-3.7))
print("trunc:", math.trunc(3.7), math.trunc(-3.7))

print("gcd:", math.gcd(12, 18), math.gcd(), math.gcd(0, 5))
print("lcm:", math.lcm(4, 6), math.lcm(), math.lcm(3, 0))
print("factorial5:", math.factorial(5))
print("factorial20:", math.factorial(20))
print("isqrt:", math.isqrt(17), math.isqrt(0))

print("isnan:", math.isnan(math.nan), math.isnan(1.0))
print("isinf:", math.isinf(math.inf), math.isinf(1.0))
print("isfinite:", math.isfinite(1.0), math.isfinite(math.inf))

print("hypot:", math.hypot(3, 4))
print("copysign:", math.copysign(3, -1))
print("fmod:", math.fmod(7, 3))
print("remainder:", math.remainder(7, 3))
print("pow:", math.pow(2, 10), math.pow(4, 0.5))
print("degrees:", round(math.degrees(math.pi), 12))
print("radians:", round(math.radians(180), 12))

print("modf:", math.modf(3.5))
print("frexp:", math.frexp(8))
print("ldexp:", math.ldexp(0.5, 4))


def show(fn):
    try:
        fn()
    except (ValueError, OverflowError) as e:
        print(type(e).__name__ + ":", e)


show(lambda: math.sqrt(-1))
show(lambda: math.log(0))
show(lambda: math.acos(2))
show(lambda: math.pow(0, -1))
show(lambda: math.exp(10000))
show(lambda: math.factorial(-1))
show(lambda: math.isqrt(-1))
