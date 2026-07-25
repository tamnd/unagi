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
for p, mu, sigma in cases:
    print(repr(_statistics._normal_dist_inv_cdf(p, mu, sigma)))

# NormalDist.inv_cdf routes through the accelerator; confirm it agrees.
nd = NormalDist(100.0, 15.0)
print("inv_cdf(0.5):", repr(nd.inv_cdf(0.5)))
print("inv_cdf(0.95):", repr(nd.inv_cdf(0.95)))
print("quantiles:", [round(q, 6) for q in nd.quantiles(4)])
