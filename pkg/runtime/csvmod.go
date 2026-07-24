package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _csv is a C accelerator in CPython, so the runtime provides it in Go. The
// pure-Python csv module is `from _csv import ...` for its whole engine: the
// reader and writer objects, the Dialect type that validates format parameters,
// the dialect registry, the Error exception, the field-size limit, and the
// QUOTE_* constants. csv.py binds reader and writer at import time, so the full
// surface has to exist at once for `import csv` to work.
//
// The parsing and formatting live in pkg/objects next to the reader, writer and
// dialect types. This file registers the module, builds the Error class, owns
// the name-to-dialect registry, and resolves a dialect argument (a name, a
// dialect object, or format keywords) before handing it to the object layer.

// csvErrorClass is _csv.Error, a subclass of Exception. The reader, writer and
// Sniffer raise it and callers catch it with `except csv.Error`. It is built in
// initCsv and threaded onto each reader and writer.
var csvErrorClass objects.Object

// csvDialects is the name-to-dialect registry, _csv's per-module dialects dict.
// It starts empty; csv.py registers excel, excel-tab and unix itself at import.
var csvDialects map[string]objects.Object

func init() {
	moduleTable["_csv"] = &moduleEntry{builtin: true, exec: initCsv}
}

func initCsv(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_csv: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("Error", "_csv.Error", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	csvErrorClass = errCls
	csvDialects = map[string]objects.Object{}

	if err := objects.StoreAttr(m, "Error", errCls); err != nil {
		return err
	}

	// The quoting styles, the integer constants csv.py re-exports.
	consts := []struct {
		name string
		val  int64
	}{
		{"QUOTE_MINIMAL", 0},
		{"QUOTE_ALL", 1},
		{"QUOTE_NONNUMERIC", 2},
		{"QUOTE_NONE", 3},
		{"QUOTE_STRINGS", 4},
		{"QUOTE_NOTNULL", 5},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	// reader(iterable, dialect='excel', **fmtparams): parse the iterable's lines
	// into rows. The dialect defaults to the built-in format, which matches
	// excel, so an unregistered default name is never needed.
	reader := objects.NewFuncKw("reader", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "reader expected at least 1 argument, got 0")
		}
		if len(pos) > 2 {
			return nil, objects.Raise(objects.TypeError, "reader expected at most 2 arguments, got %d", len(pos))
		}
		var base objects.Object
		if len(pos) == 2 {
			base = pos[1]
		}
		d, err := csvMakeDialect(base, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return objects.NewCsvReader(pos[0], d, csvErrorClass)
	})
	if err := objects.StoreAttr(m, "reader", reader); err != nil {
		return err
	}

	// writer(fileobj, dialect='excel', **fmtparams): format rows and write them
	// to a file object that exposes a write method.
	writer := objects.NewFuncKw("writer", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "writer expected at least 1 argument, got 0")
		}
		if len(pos) > 2 {
			return nil, objects.Raise(objects.TypeError, "writer expected at most 2 arguments, got %d", len(pos))
		}
		var base objects.Object
		if len(pos) == 2 {
			base = pos[1]
		}
		d, err := csvMakeDialect(base, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return objects.NewCsvWriter(pos[0], d, csvErrorClass)
	})
	if err := objects.StoreAttr(m, "writer", writer); err != nil {
		return err
	}

	// Dialect(dialect=None, **fmtparams): build and validate a dialect object.
	// csv.py uses it as _Dialect(self) to validate its own Dialect classes.
	dialect := objects.NewFuncKw("Dialect", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) > 1 {
			return nil, objects.Raise(objects.TypeError, "Dialect() takes at most 1 argument (%d given)", len(pos))
		}
		var base objects.Object
		if len(pos) == 1 {
			base = pos[0]
		}
		return csvMakeDialect(base, kwNames, kwVals)
	})
	if err := objects.StoreAttr(m, "Dialect", dialect); err != nil {
		return err
	}

	// register_dialect(name, dialect=None, **fmtparams): validate a dialect and
	// bind it to a name in the registry.
	registerDialect := objects.NewFuncKw("register_dialect", func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
		if len(pos) < 1 {
			return nil, objects.Raise(objects.TypeError, "register_dialect() takes at least 1 argument (0 given)")
		}
		if len(pos) > 2 {
			return nil, objects.Raise(objects.TypeError, "register_dialect() takes at most 2 arguments (%d given)", len(pos))
		}
		name, ok := objects.AsStr(pos[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "dialect name must be a string")
		}
		var base objects.Object
		if len(pos) == 2 {
			base = pos[1]
		}
		d, err := csvMakeDialect(base, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		csvDialects[name] = d
		return objects.None, nil
	})
	if err := objects.StoreAttr(m, "register_dialect", registerDialect); err != nil {
		return err
	}

	unregisterDialect := objects.NewFunc("unregister_dialect", 1, func(args []objects.Object) (objects.Object, error) {
		name, ok := objects.AsStr(args[0])
		if !ok {
			return nil, csvError("unknown dialect")
		}
		if _, present := csvDialects[name]; !present {
			return nil, csvError("unknown dialect")
		}
		delete(csvDialects, name)
		return objects.None, nil
	})
	if err := objects.StoreAttr(m, "unregister_dialect", unregisterDialect); err != nil {
		return err
	}

	getDialect := objects.NewFunc("get_dialect", 1, func(args []objects.Object) (objects.Object, error) {
		return csvLookupDialect(args[0])
	})
	if err := objects.StoreAttr(m, "get_dialect", getDialect); err != nil {
		return err
	}

	listDialects := objects.NewFunc("list_dialects", 0, func(args []objects.Object) (objects.Object, error) {
		names := make([]objects.Object, 0, len(csvDialects))
		for name := range csvDialects {
			names = append(names, objects.NewStr(name))
		}
		return objects.NewList(names), nil
	})
	if err := objects.StoreAttr(m, "list_dialects", listDialects); err != nil {
		return err
	}

	// field_size_limit(new_limit=None): read, and optionally set, the field cap.
	fieldSizeLimit := objects.NewFunc("field_size_limit", -1, func(args []objects.Object) (objects.Object, error) {
		if len(args) > 1 {
			return nil, objects.Raise(objects.TypeError, "field_size_limit() takes at most 1 argument (%d given)", len(args))
		}
		old := objects.CsvFieldLimit()
		if len(args) == 1 {
			// PyLong_CheckExact: a plain int, so a bool is rejected.
			if args[0].TypeName() != "int" {
				return nil, objects.Raise(objects.TypeError, "limit must be an integer")
			}
			n, _ := objects.AsInt(args[0])
			objects.CsvSetFieldLimit(n)
		}
		return objects.NewInt(old), nil
	})
	return objects.StoreAttr(m, "field_size_limit", fieldSizeLimit)
}

// csvMakeDialect resolves a dialect argument into a validated dialect. The base
// may be a positional value or a "dialect" keyword; a string base is looked up
// in the registry, None means all defaults, and any other object is read for its
// attributes. The remaining format keywords override the base.
func csvMakeDialect(base objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	fmtNames := make([]string, 0, len(kwNames))
	fmtVals := make([]objects.Object, 0, len(kwVals))
	for i, k := range kwNames {
		if k == "dialect" {
			if base == nil {
				base = kwVals[i]
			}
			continue
		}
		fmtNames = append(fmtNames, k)
		fmtVals = append(fmtVals, kwVals[i])
	}
	if base != nil {
		if name, ok := objects.AsStr(base); ok {
			resolved, err := csvLookupDialect(objects.NewStr(name))
			if err != nil {
				return nil, err
			}
			base = resolved
		} else if base == objects.None {
			base = nil
		}
	}
	return objects.NewCsvDialect(base, fmtNames, fmtVals)
}

// csvLookupDialect returns the registered dialect for a name, or raises the
// _csv.Error "unknown dialect" the registry lookups all share.
func csvLookupDialect(nameObj objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(nameObj)
	if !ok {
		return nil, csvError("unknown dialect")
	}
	d, present := csvDialects[name]
	if !present {
		return nil, csvError("unknown dialect")
	}
	return d, nil
}

// csvError raises a _csv.Error carrying the message, for the registry paths that
// do not go through the object layer's csvErrorf.
func csvError(msg string) error {
	if csvErrorClass != nil {
		if inst, err := objects.Call(csvErrorClass, []objects.Object{objects.NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return objects.Raise(objects.RuntimeError, "%s", msg)
}
