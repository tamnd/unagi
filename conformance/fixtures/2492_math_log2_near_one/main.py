import math
# log2 near 1 is where Go's frexp-based log2 lost precision. Format to 12
# significant figures, coarser than the last-ulp libm divergence but fine enough
# to catch the old catastrophic-cancellation error, so this is a stable oracle.
xs = [1.0000000000000002, 1.0000000001, 1.00001, 1.0001, 0.9999, 0.99999, 1.5, 0.75]
for x in xs:
    print("log2(%r) = %s" % (x, format(math.log2(x), ".12g")))
# Exact powers of two stay exact integers.
for e in range(-5, 6):
    x = 2.0 ** e
    print("log2(2**%d) = %r" % (e, math.log2(x)))
# Large-int log2 stays finite and matches.
print("log2(2**1000) = %s" % format(math.log2(2**1000), ".12g"))
print("log2(3**500) = %s" % format(math.log2(3**500), ".12g"))
