package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestStatisticsInvCDF checks the accelerator imports and reproduces CPython's
// _normal_dist_inv_cdf across every branch of the AS241 approximation. The
// expected values are CPython 3.14.6 output; the arithmetic is deterministic
// IEEE-754, so equality is exact.
func TestStatisticsInvCDF(t *testing.T) {
	mo, err := ImportModule("_statistics")
	if err != nil {
		t.Fatalf("import _statistics: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "_normal_dist_inv_cdf")
	if err != nil {
		t.Fatalf("_normal_dist_inv_cdf: %v", err)
	}
	cases := []struct {
		p, mu, sigma, want float64
	}{
		{0.5, 0.0, 1.0, 0.0},
		{0.975, 0.0, 1.0, 1.9599639845400534},
		{0.001, 2.0, 3.0, -7.27069691850344},
		{0.25, 10.0, 2.0, 8.651020499607837},
		{0.999999, 0.0, 1.0, 4.753424308817086},
		{1e-9, 0.0, 1.0, -5.997807015007685},
	}
	for _, c := range cases {
		got, err := objects.Call(fn, []objects.Object{
			objects.NewFloat(c.p), objects.NewFloat(c.mu), objects.NewFloat(c.sigma),
		})
		if err != nil {
			t.Fatalf("inv_cdf(%v): %v", c, err)
		}
		g, ok := objects.AsFloat(got)
		if !ok || g != c.want {
			t.Fatalf("inv_cdf(%v, %v, %v) = %v, want %v", c.p, c.mu, c.sigma, got, c.want)
		}
	}
}
