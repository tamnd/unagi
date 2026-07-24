# sys.float_info is the structseq of C double limits. statistics reads mant_dig
# at import to size its exact-sum accumulator, so the module imports once this is
# present. The values are the IEEE 754 binary64 limits, identical on every host.
import sys

fi = sys.float_info
print(fi)
print(fi.max, fi.min, fi.epsilon)
print(fi.max_exp, fi.max_10_exp, fi.min_exp, fi.min_10_exp)
print(fi.dig, fi.mant_dig, fi.radix, fi.rounds)

# It behaves as a structseq: indexable, sized, and equal to itself.
print(len(fi))
print(fi[0] == fi.max, fi[7] == fi.mant_dig, fi[10] == fi.rounds)
print(type(fi).__name__)
print(tuple(fi) == (fi.max, fi.max_exp, fi.max_10_exp, fi.min, fi.min_exp,
                    fi.min_10_exp, fi.dig, fi.mant_dig, fi.epsilon, fi.radix, fi.rounds))

# The exact edges match float's own limits.
print(fi.max == 1.7976931348623157e+308)
print(fi.epsilon == 2.220446049250313e-16)
print(1.0 + fi.epsilon != 1.0)

# statistics imports now and its non-Fraction paths run.
import statistics
print("statistics imported")
print(statistics.median([1, 3, 5, 7]))
print(statistics.median_low([1, 2, 3, 4]), statistics.median_high([1, 2, 3, 4]))
print(statistics.mode([1, 1, 2, 3]))
