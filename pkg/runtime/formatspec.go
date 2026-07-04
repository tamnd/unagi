package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// FormatSpec formats a value with a spec that was itself built at runtime,
// which happens when an f-string spec contains replacement fields like
// f"{x:{w}}". The spec object is always a str produced by the surrounding
// lowering, so its text drives objects.Format the same way a literal spec
// would. Probed on 3.14: format(42, "6") and f"{42:{6}}" both give "    42".
func FormatSpec(o, spec objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(spec)
	if !ok {
		s = objects.Str(spec)
	}
	return objects.Format(o, s)
}
