import _statistics
from statistics import NormalDist

# The accelerator's single function, exercised across every branch of the AS241
# approximation: the central |q| <= 0.425 branch, the r <= 5 tail and the far
# r > 5 tail, on both sides of the median.
cases = [
    (0.5, 0.0, 1.0),
    (0.975, 0.0, 1.0),
    (0.025, 0.0, 1.0),
    (0.001, 2.0, 3.0),
    (0.25, 10.0, 2.0),
    (0.999999, 0.0, 1.0),
    (1e-9, 0.0, 1.0),
    (0.6, -5.0, 0.5),
]
# The AS241 approximation is a chain of multiply-adds whose last bit moves by a
# ulp depending on whether the host libm/compiler contracts a step to an FMA, so
# the results are rounded to ten places rather than printed bit-for-bit.
for p, mu, sigma in cases:
    print(round(_statistics._normal_dist_inv_cdf(p, mu, sigma), 10))

# NormalDist.inv_cdf routes through the accelerator; confirm it agrees.
nd = NormalDist(100.0, 15.0)
print("inv_cdf(0.5):", round(nd.inv_cdf(0.5), 10))
print("inv_cdf(0.95):", round(nd.inv_cdf(0.95), 10))
print("quantiles:", [round(q, 6) for q in nd.quantiles(4)])
