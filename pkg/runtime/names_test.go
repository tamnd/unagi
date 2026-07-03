package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// Every expected message below was probed against python3.14 (3.14.6):
// an unassigned or deleted local reads and deletes as UnboundLocalError
// with the "cannot access local variable ..." text, an undefined module
// name as NameError, and except NameError catches UnboundLocalError.

func TestLoadLocal(t *testing.T) {
	v := objects.NewInt(7)
	got, err := LoadLocal(v, "x")
	if err != nil || got != v {
		t.Errorf("LoadLocal(bound) = %v, %v", got, err)
	}
	_, err = LoadLocal(nil, "x")
	checkErr(t, "unbound local", err,
		"UnboundLocalError: cannot access local variable 'x' where it is not associated with a value")
	// Probed on 3.14: except NameError catches UnboundLocalError, and
	// issubclass(UnboundLocalError, NameError) is True.
	if !ExcMatches(err, "NameError") {
		t.Error("UnboundLocalError does not match except NameError")
	}
	if !objects.Matches("UnboundLocalError", "NameError") {
		t.Error("UnboundLocalError is not a NameError subclass in the table")
	}
}

func TestLoadName(t *testing.T) {
	v := objects.NewStr("mod")
	got, err := LoadName(v, "x")
	if err != nil || got != v {
		t.Errorf("LoadName(bound) = %v, %v", got, err)
	}
	_, err = LoadName(nil, "nosuchname")
	checkErr(t, "undefined name", err, "NameError: name 'nosuchname' is not defined")
}

func TestDelLocalAndDelName(t *testing.T) {
	if err := DelLocal(objects.None, "x"); err != nil {
		t.Errorf("DelLocal(bound) = %v", err)
	}
	// Probed on 3.14: del x before assignment gives the same text as a read.
	err := DelLocal(nil, "x")
	checkErr(t, "del unbound local", err,
		"UnboundLocalError: cannot access local variable 'x' where it is not associated with a value")

	if err := DelName(objects.False, "x"); err != nil {
		t.Errorf("DelName(bound) = %v", err)
	}
	// Probed on 3.14: del zzz_undefined at module scope.
	err = DelName(nil, "zzz_undefined")
	checkErr(t, "del undefined name", err, "NameError: name 'zzz_undefined' is not defined")
}
