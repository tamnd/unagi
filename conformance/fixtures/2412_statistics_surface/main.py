# The statistics module runs on the vendored statistics.py over the _statistics
# accelerator: the descriptive functions (the means, the medians, the spread and
# the quantiles), the correlation family, the NormalDist class and its arithmetic,
# and the kde estimator all match CPython. Irrational results are rounded because
# they ride the same Go-vs-libm last-ULP ceiling the math and cmath ports document,
# and the exact-arithmetic results (a Fraction mean, an int variance) print in full.
import statistics as s
from statistics import NormalDist as N
from fractions import Fraction as F


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


# The mean family: an int mean stays exact, a Fraction mean stays a Fraction, and
# fmean, geometric_mean and harmonic_mean carry their weighted forms.
show("mean", lambda: s.mean([1, 2, 3, 4]))
show("mean_frac", lambda: s.mean([F(1, 2), F(1, 3)]))
show("fmean", lambda: s.fmean([1, 2, 3, 4]))
show("fmean_weighted", lambda: s.fmean([1, 2, 3], [3, 2, 1]))
show("geometric_mean", lambda: round(s.geometric_mean([2, 4, 8]), 10))
show("harmonic_mean", lambda: round(s.harmonic_mean([2, 4, 8]), 10))
show("harmonic_mean_weighted", lambda: s.harmonic_mean([40, 60], [5, 30]))

# The median family and the modes.
show("median", lambda: s.median([1, 2, 3, 4]))
show("median_low", lambda: s.median_low([1, 2, 3, 4]))
show("median_high", lambda: s.median_high([1, 2, 3, 4]))
show("median_grouped", lambda: s.median_grouped([1, 2, 2, 3, 4, 4, 4, 4, 4, 5]))
show("mode", lambda: s.mode([1, 1, 2, 3]))
show("multimode", lambda: s.multimode("aabbbccde"))

# The spread: a population variance over ints stays exact, the sample forms round.
show("pvariance", lambda: s.pvariance([1, 2, 3, 4, 5]))
show("pstdev", lambda: round(s.pstdev([1, 2, 3, 4, 5]), 10))
show("variance", lambda: s.variance([1, 2, 3, 4, 5]))
show("stdev", lambda: round(s.stdev([1, 2, 3, 4, 5]), 10))

# The quantiles, both cut-point methods, and the correlation family.
show("quantiles", lambda: s.quantiles([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]))
show("quantiles_inclusive", lambda: s.quantiles([1, 2, 3, 4], method="inclusive"))
show("correlation", lambda: s.correlation([1, 2, 3, 4, 5], [2, 4, 6, 8, 10]))
show("covariance", lambda: s.covariance([1, 2, 3, 4, 5], [2, 4, 6, 8, 10]))
show("linear_regression", lambda: s.linear_regression([1, 2, 3], [2, 4, 6]))

# NormalDist: the attributes, the constructors, the pdf/cdf/inv_cdf, the derived
# measures, the arithmetic and the value equality.
nd = N(2, 3)
show("nd_repr", lambda: repr(nd))
show("nd_attrs", lambda: (nd.mean, nd.median, nd.mode, nd.stdev, nd.variance))
show("nd_from_samples", lambda: N.from_samples([1, 2, 3, 4, 5]))
show("nd_pdf", lambda: round(N(0, 1).pdf(0.5), 10))
show("nd_cdf", lambda: round(N(0, 1).cdf(1.0), 10))
show("nd_inv_cdf", lambda: round(N(0, 1).inv_cdf(0.975), 10))
show("nd_overlap", lambda: round(N(0, 1).overlap(N(1, 1)), 10))
show("nd_zscore", lambda: N(0, 1).zscore(1.5))
show("nd_quantiles", lambda: [round(q, 6) for q in N(0, 1).quantiles(4)])
show("nd_add", lambda: N(1, 2) + N(3, 4))
show("nd_sub", lambda: N(5, 2) - N(1, 1))
show("nd_mul", lambda: N(2, 3) * 2)
show("nd_div", lambda: N(4, 2) / 2)
show("nd_eq", lambda: N(1, 2) == N(1, 2))
show("nd_hash_eq", lambda: hash(N(1, 2)) == hash(N(1, 2)))

# The kde estimator across its kernels, its cumulative form, and kde_random.
data = [1.0, 2.0, 3.0, 4.0, 5.0, 6.0]
show("kde_normal", lambda: round(s.kde(data, h=1.5)(3.0), 10))
show("kde_logistic", lambda: round(s.kde(data, h=1.5, kernel="logistic")(3.0), 10))
show("kde_epanechnikov", lambda: round(s.kde(data, h=1.5, kernel="epanechnikov")(3.0), 10))
show("kde_cumulative", lambda: round(s.kde(data, h=1.5, cumulative=True)(3.0), 10))
show("kde_random_len", lambda: len([s.kde_random(data, h=1.5, seed=42)() for _ in range(3)]))

# The error paths: an empty mean, a single-point sample variance, an empty mode, a
# negative harmonic input, a zero-cut quantile, a mismatched correlation, a
# non-positive sigma and a bad kde kernel each report CPython's message.
show("mean_empty", lambda: s.mean([]))
show("variance_one", lambda: s.variance([1]))
show("mode_empty", lambda: s.mode([]))
show("harmonic_negative", lambda: s.harmonic_mean([1, -1]))
show("quantiles_n0", lambda: s.quantiles([1, 2, 3], n=0))
show("correlation_mismatch", lambda: s.correlation([1, 2], [1, 2, 3]))
show("nd_zero_sigma", lambda: N(0, 0))
show("kde_bad_kernel", lambda: s.kde(data, h=1, kernel="nope"))
