package lower

import "github.com/tamnd/unagi/pkg/frontend"

// typeAlias lowers a PEP 695 `type Name = value` statement. CPython evaluates
// the value lazily, only when TypeAliasType.__value__ is read, and it may name
// the alias itself for a recursive alias, so it lowers as a zero-argument lambda
// closing over the enclosing scope. NewTypeAlias holds that compute callable and
// forces it once on the first __value__ access. The alias name binds through the
// ordinary assignment path, so it lands in the module, a class namespace, or a
// local exactly as a plain assignment would. Type parameters are erased, so the
// alias reports an empty __type_params__.
func (f *fnCtx) typeAlias(s *frontend.TypeAlias) error {
	compute, err := f.lambda(&frontend.Lambda{Pos_: s.Pos_, Body: s.Value})
	if err != nil {
		return err
	}
	alias := callExpr(f.e.obj("NewTypeAlias"), strLit(s.Name), compute)
	return f.assignTo(&frontend.Name{Pos_: s.Pos_, Id: s.Name}, alias)
}
