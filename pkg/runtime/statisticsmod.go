package runtime

import (
	"math"

	"github.com/tamnd/unagi/pkg/objects"
)

// _statistics is the accelerator behind the public statistics module. In
// CPython 3.14 it exposes a single function, _normal_dist_inv_cdf, the inverse
// cumulative distribution function of the normal distribution that NormalDist
// uses. statistics.py ships a pure-Python fallback, so `import statistics`
// already worked; this makes `import _statistics` itself work and gives
// NormalDist the C-path result.
//
// The computation is Wichura's AS241 rational approximation, the same algorithm
// statistics.py implements in Python. It is pure IEEE-754 double arithmetic, so
// evaluating it in Go float64 in the same operation order is byte-identical to
// CPython.
//
// The module is portable, so it registers on every target.

func init() {
	moduleTable["_statistics"] = &moduleEntry{builtin: true, exec: initStatistics}
}

func initStatistics(m *objects.Module) error {
	return objects.StoreAttr(m, "_normal_dist_inv_cdf",
		objects.NewFunc("_normal_dist_inv_cdf", 3, statisticsNormalInvCDF))
}

// statisticsNormalInvCDF is _statistics._normal_dist_inv_cdf(p, mu, sigma).
func statisticsNormalInvCDF(args []objects.Object) (objects.Object, error) {
	p, ok := objects.AsFloat(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[0].TypeName())
	}
	mu, ok := objects.AsFloat(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[1].TypeName())
	}
	sigma, ok := objects.AsFloat(args[2])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[2].TypeName())
	}
	return objects.NewFloat(normalDistInvCDF(p, mu, sigma)), nil
}

// normalDistInvCDF is a direct port of statistics._normal_dist_inv_cdf: Wichura's
// AS241 rational approximation. The operation order matches the Python source so
// the float64 result is byte-identical.
func normalDistInvCDF(p, mu, sigma float64) float64 {
	q := p - 0.5

	var num, den, x float64
	if math.Abs(q) <= 0.425 {
		r := 0.180625 - q*q
		num = (((((((2.5090809287301226727e+3*r+
			3.3430575583588128105e+4)*r+
			6.7265770927008700853e+4)*r+
			4.5921953931549871457e+4)*r+
			1.3731693765509461125e+4)*r+
			1.9715909503065514427e+3)*r+
			1.3314166789178437745e+2)*r+
			3.3871328727963666080e+0) * q
		den = (((((((5.2264952788528545610e+3*r+
			2.8729085735721942674e+4)*r+
			3.9307895800092710610e+4)*r+
			2.1213794301586595867e+4)*r+
			5.3941960214247511077e+3)*r+
			6.8718700749205790830e+2)*r+
			4.2313330701600911252e+1)*r +
			1.0)
		x = num / den
		return mu + (x * sigma)
	}

	var r float64
	if q <= 0.0 {
		r = p
	} else {
		r = 1.0 - p
	}
	r = math.Sqrt(-math.Log(r))
	if r <= 5.0 {
		r = r - 1.6
		num = (((((((7.7454501427834140764e-4*r+
			2.2723844989269184583e-2)*r+
			2.4178072517745061177e-1)*r+
			1.2704582524523683826e+0)*r+
			3.6478483247632046050e+0)*r+
			5.7694972214606914055e+0)*r+
			4.6303378461565452959e+0)*r +
			1.4234371107496835773e+0)
		den = (((((((1.0507500716444168432e-9*r+
			5.4759380849953449460e-4)*r+
			1.5198666563616457197e-2)*r+
			1.4810397642748007459e-1)*r+
			6.8976733498510000455e-1)*r+
			1.6763848301838038494e+0)*r+
			2.0531916266377588219e+0)*r +
			1.0)
	} else {
		r = r - 5.0
		num = (((((((2.0103343992922881327e-7*r+
			2.7115555687434875782e-5)*r+
			1.2426609473880784386e-3)*r+
			2.6532189526576123093e-2)*r+
			2.9656057182850489123e-1)*r+
			1.7848265399172913358e+0)*r+
			5.4637849111641143699e+0)*r +
			6.6579046435011037772e+0)
		den = (((((((2.0442631033899397856e-15*r+
			1.4215117583164458887e-7)*r+
			1.8463183175100546818e-5)*r+
			7.8686913114561325910e-4)*r+
			1.4875361290850614853e-2)*r+
			1.3692988092273580531e-1)*r+
			5.9983220655588793769e-1)*r +
			1.0)
	}

	x = num / den
	if q < 0.0 {
		x = -x
	}
	return mu + (x * sigma)
}
