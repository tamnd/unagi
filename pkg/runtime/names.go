package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Name checks. The emitter models every variable as an objects.Object
// that is nil before its first assignment and nil again after del, so
// these helpers turn a nil load or delete into the right Python error.
// The clearing itself stays in emitted code: after a Del check passes,
// the emitter assigns nil to the variable.

// unboundLocal is the error for touching an unassigned function local.
// Probed on 3.14: reading x before assignment inside a function, and
// del x before assignment, both give exactly this text.
func unboundLocal(name string) error {
	return objects.Raise(objects.UnboundLocalError,
		"cannot access local variable '%s' where it is not associated with a value", name)
}

// nameNotDefined is the error for touching an undefined module-scope
// name. Probed on 3.14: nosuchname and del nosuchname at module scope.
func nameNotDefined(name string) error {
	return objects.Raise(objects.NameError, "name '%s' is not defined", name)
}

// LoadLocal returns a function-local variable, or the UnboundLocalError
// CPython raises for a read before assignment or after del.
func LoadLocal(v objects.Object, name string) (objects.Object, error) {
	if v != nil {
		return v, nil
	}
	return nil, unboundLocal(name)
}

// LoadName is the module-scope variant of LoadLocal, raising NameError.
func LoadName(v objects.Object, name string) (objects.Object, error) {
	if v != nil {
		return v, nil
	}
	return nil, nameNotDefined(name)
}

// LoadFree returns a variable captured from an enclosing function, or the
// NameError CPython raises when the closure slot is still empty. Probed on
// 3.14: a lambda reading an outer local before its assignment.
func LoadFree(v objects.Object, name string) (objects.Object, error) {
	if v != nil {
		return v, nil
	}
	return nil, objects.Raise(objects.NameError,
		"cannot access free variable '%s' where it is not associated with a value in enclosing scope", name)
}

// DelLocal checks `del x` on a function local that may be unbound.
func DelLocal(v objects.Object, name string) error {
	if v == nil {
		return unboundLocal(name)
	}
	return nil
}

// DelName checks `del x` on a module-scope name that may be undefined.
func DelName(v objects.Object, name string) error {
	if v == nil {
		return nameNotDefined(name)
	}
	return nil
}
