package objects

import "testing"

// Every expectation below was probed against python3.14 (3.14.6): the
// chaining dunders read off the exception's slots, assigning __cause__ also
// sets __suppress_context__, and a non-exception cause or non-bool suppress
// is a TypeError.

func TestExcChainingReads(t *testing.T) {
	cause := Raise(KeyError, "k")
	e := &Exception{Kind: RuntimeError, Args: []Object{NewStr("x")}, Cause: cause, SuppressContext: true}

	got, err := LoadAttr(e, "__cause__")
	if err != nil || got != cause {
		t.Errorf("__cause__ = %v, %v", got, err)
	}
	got, _ = LoadAttr(e, "__context__")
	if got != None {
		t.Errorf("__context__ with no context = %v, want None", got)
	}
	got, _ = LoadAttr(e, "__suppress_context__")
	if got != True {
		t.Errorf("__suppress_context__ = %v, want True", got)
	}
	got, _ = LoadAttr(e, "__traceback__")
	if got != None {
		t.Errorf("__traceback__ = %v, want None", got)
	}
}

func TestExcNotesReadNeedsAddNote(t *testing.T) {
	e := Raise(ValueError, "x")
	_, err := LoadAttr(e, "__notes__")
	checkErr(t, "no notes", err, "AttributeError: 'ValueError' object has no attribute '__notes__'")

	e.Notes = append(e.Notes, "first")
	got, err := LoadAttr(e, "__notes__")
	if err != nil {
		t.Fatalf("__notes__ after add = %v", err)
	}
	if Repr(got) != "['first']" {
		t.Errorf("__notes__ = %s, want ['first']", Repr(got))
	}
}

func TestExcGroupAttrs(t *testing.T) {
	sub := []*Exception{Raise(ValueError, "a"), Raise(KeyError, "b")}
	objs := []Object{sub[0], sub[1]}
	g := &Exception{Kind: "ExceptionGroup", Args: []Object{NewStr("group msg"), NewList(objs)}, Group: sub}

	msg, err := LoadAttr(g, "message")
	if err != nil || Str(msg) != "group msg" {
		t.Errorf("message = %v, %v", msg, err)
	}
	exc, err := LoadAttr(g, "exceptions")
	if err != nil {
		t.Fatalf("exceptions = %v", err)
	}
	if Repr(exc) != "(ValueError('a'), KeyError('b'))" {
		t.Errorf("exceptions = %s", Repr(exc))
	}

	// A plain exception has neither attribute.
	plain := Raise(ValueError, "x")
	_, err = LoadAttr(plain, "message")
	checkErr(t, "plain message", err, "AttributeError: 'ValueError' object has no attribute 'message'")
	_, err = LoadAttr(plain, "exceptions")
	checkErr(t, "plain exceptions", err, "AttributeError: 'ValueError' object has no attribute 'exceptions'")
}

func TestExcCauseWriteSetsSuppress(t *testing.T) {
	e := Raise(ValueError, "x")
	cause := Raise(KeyError, "k")
	if err := StoreAttr(e, "__cause__", cause); err != nil {
		t.Fatalf("set __cause__ = %v", err)
	}
	if e.Cause != cause || !e.SuppressContext {
		t.Errorf("after __cause__ set: cause=%v suppress=%v", e.Cause, e.SuppressContext)
	}
	// Even a None cause leaves suppression on, matching CPython.
	if err := StoreAttr(e, "__cause__", None); err != nil {
		t.Fatalf("clear __cause__ = %v", err)
	}
	if e.Cause != nil || !e.SuppressContext {
		t.Errorf("after None cause: cause=%v suppress=%v", e.Cause, e.SuppressContext)
	}
}

func TestExcContextWriteKeepsSuppress(t *testing.T) {
	e := Raise(ValueError, "x")
	ctx := Raise(KeyError, "k")
	if err := StoreAttr(e, "__context__", ctx); err != nil {
		t.Fatalf("set __context__ = %v", err)
	}
	if e.Context != ctx || e.SuppressContext {
		t.Errorf("after __context__ set: context=%v suppress=%v", e.Context, e.SuppressContext)
	}
}

func TestExcAttrWriteRejects(t *testing.T) {
	e := Raise(ValueError, "x")
	err := StoreAttr(e, "__cause__", NewInt(5))
	checkErr(t, "bad cause", err, "TypeError: exception cause must be None or derive from BaseException")
	err = StoreAttr(e, "__context__", NewInt(5))
	checkErr(t, "bad context", err, "TypeError: exception context must be None or derive from BaseException")
	err = StoreAttr(e, "__suppress_context__", NewStr("yes"))
	checkErr(t, "bad suppress", err, "TypeError: attribute value type must be bool")
}
