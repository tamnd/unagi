package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _sqlite3 is the C accelerator sqlite3/dbapi2.py opens with `from _sqlite3
// import *`. The module was missing, so `import sqlite3` failed with
// ModuleNotFoundError.
//
// _sqlite3 wraps the SQLite C library. The Go standard library carries no SQLite
// engine (database/sql needs an out-of-tree driver, and there is no pure-Go
// SQLite in std), so under the stdlib-only, no-cgo constraint there is no engine
// to back a real Connection, the same honest gap as _lzma (#789), where Go's
// standard library has no xz codec.
//
// But the import-time surface is not the engine. dbapi2.py reads only the
// SQLITE_* constant block, the exception classes, sqlite_version (to build
// sqlite_version_info), the Row type (collections.abc.Sequence.register(Row) at
// module scope), and calls register_adapter/register_converter at module scope
// through register_adapters_and_converters(). It builds a Connection only inside
// connect(), never at import. So the constants, the real exception hierarchy, the
// Row/PrepareProtocol types, and present-but-inert register_* let `import
// sqlite3` succeed, while connect() and the value adapters raise cleanly at the
// call rather than fabricating a database. This is the reduced-surface stance
// _lzma (#789) and _symtable (#788) take.
//
// The constant values are CPython 3.14.6's, fixed by SQLite's C headers (result
// codes, authorizer actions, limit ids, dbconfig ids), so they are
// platform-stable by construction. threadsafety is 0 (this build shares no real
// connection across threads) and sqlite_version is "0.0.0" because no SQLite
// library is linked, rather than claiming a version this build cannot honor.

func init() {
	moduleTable["_sqlite3"] = &moduleEntry{builtin: true, exec: initSqlite3}
}

// sqlite3Unavailable is the error every operation that needs the SQLite engine
// raises: there is no SQLite backend in this build, so it cannot run rather than
// fabricate a database.
func sqlite3Unavailable() error {
	return objects.Raise("NotImplementedError",
		"sqlite3 is not available: this build carries no SQLite engine (Go's standard library has no SQLite driver)")
}

func initSqlite3(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_sqlite3: Exception base is unavailable")
	}

	// The DB-API 2.0 exception hierarchy SQLite errors surface as, built as real
	// catchable classes so `except sqlite3.OperationalError` and the like bind.
	// Warning and Error subclass Exception; the rest chain under Error /
	// DatabaseError, matching CPython.
	mkExc := func(name string, base objects.Object) (objects.Object, error) {
		cls, err := objects.NewClass(name, "sqlite3."+name, []objects.Object{base}, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		return cls, objects.StoreAttr(m, name, cls)
	}
	if _, err := mkExc("Warning", exc); err != nil {
		return err
	}
	errCls, err := mkExc("Error", exc)
	if err != nil {
		return err
	}
	if _, err := mkExc("InterfaceError", errCls); err != nil {
		return err
	}
	dbErr, err := mkExc("DatabaseError", errCls)
	if err != nil {
		return err
	}
	for _, name := range []string{"DataError", "OperationalError", "IntegrityError", "InternalError", "ProgrammingError", "NotSupportedError"} {
		if _, err := mkExc(name, dbErr); err != nil {
			return err
		}
	}

	// The SQLite constant block: result and extended result codes, authorizer
	// action codes, limit ids, and dbconfig ids, plus the parse flags and
	// threadsafety. These are real and usable regardless of the missing engine.
	consts := []struct {
		name string
		val  int64
	}{
		{"LEGACY_TRANSACTION_CONTROL", -1},
		{"PARSE_COLNAMES", 2},
		{"PARSE_DECLTYPES", 1},
		{"SQLITE_ABORT", 4},
		{"SQLITE_ABORT_ROLLBACK", 516},
		{"SQLITE_ALTER_TABLE", 26},
		{"SQLITE_ANALYZE", 28},
		{"SQLITE_ATTACH", 24},
		{"SQLITE_AUTH", 23},
		{"SQLITE_AUTH_USER", 279},
		{"SQLITE_BUSY", 5},
		{"SQLITE_BUSY_RECOVERY", 261},
		{"SQLITE_BUSY_SNAPSHOT", 517},
		{"SQLITE_BUSY_TIMEOUT", 773},
		{"SQLITE_CANTOPEN", 14},
		{"SQLITE_CANTOPEN_CONVPATH", 1038},
		{"SQLITE_CANTOPEN_DIRTYWAL", 1294},
		{"SQLITE_CANTOPEN_FULLPATH", 782},
		{"SQLITE_CANTOPEN_ISDIR", 526},
		{"SQLITE_CANTOPEN_NOTEMPDIR", 270},
		{"SQLITE_CANTOPEN_SYMLINK", 1550},
		{"SQLITE_CONSTRAINT", 19},
		{"SQLITE_CONSTRAINT_CHECK", 275},
		{"SQLITE_CONSTRAINT_COMMITHOOK", 531},
		{"SQLITE_CONSTRAINT_FOREIGNKEY", 787},
		{"SQLITE_CONSTRAINT_FUNCTION", 1043},
		{"SQLITE_CONSTRAINT_NOTNULL", 1299},
		{"SQLITE_CONSTRAINT_PINNED", 2835},
		{"SQLITE_CONSTRAINT_PRIMARYKEY", 1555},
		{"SQLITE_CONSTRAINT_ROWID", 2579},
		{"SQLITE_CONSTRAINT_TRIGGER", 1811},
		{"SQLITE_CONSTRAINT_UNIQUE", 2067},
		{"SQLITE_CONSTRAINT_VTAB", 2323},
		{"SQLITE_CORRUPT", 11},
		{"SQLITE_CORRUPT_INDEX", 779},
		{"SQLITE_CORRUPT_SEQUENCE", 523},
		{"SQLITE_CORRUPT_VTAB", 267},
		{"SQLITE_CREATE_INDEX", 1},
		{"SQLITE_CREATE_TABLE", 2},
		{"SQLITE_CREATE_TEMP_INDEX", 3},
		{"SQLITE_CREATE_TEMP_TABLE", 4},
		{"SQLITE_CREATE_TEMP_TRIGGER", 5},
		{"SQLITE_CREATE_TEMP_VIEW", 6},
		{"SQLITE_CREATE_TRIGGER", 7},
		{"SQLITE_CREATE_VIEW", 8},
		{"SQLITE_CREATE_VTABLE", 29},
		{"SQLITE_DBCONFIG_DEFENSIVE", 1010},
		{"SQLITE_DBCONFIG_DQS_DDL", 1014},
		{"SQLITE_DBCONFIG_DQS_DML", 1013},
		{"SQLITE_DBCONFIG_ENABLE_FKEY", 1002},
		{"SQLITE_DBCONFIG_ENABLE_FTS3_TOKENIZER", 1004},
		{"SQLITE_DBCONFIG_ENABLE_LOAD_EXTENSION", 1005},
		{"SQLITE_DBCONFIG_ENABLE_QPSG", 1007},
		{"SQLITE_DBCONFIG_ENABLE_TRIGGER", 1003},
		{"SQLITE_DBCONFIG_ENABLE_VIEW", 1015},
		{"SQLITE_DBCONFIG_LEGACY_ALTER_TABLE", 1012},
		{"SQLITE_DBCONFIG_LEGACY_FILE_FORMAT", 1016},
		{"SQLITE_DBCONFIG_NO_CKPT_ON_CLOSE", 1006},
		{"SQLITE_DBCONFIG_RESET_DATABASE", 1009},
		{"SQLITE_DBCONFIG_TRIGGER_EQP", 1008},
		{"SQLITE_DBCONFIG_TRUSTED_SCHEMA", 1017},
		{"SQLITE_DBCONFIG_WRITABLE_SCHEMA", 1011},
		{"SQLITE_DELETE", 9},
		{"SQLITE_DENY", 1},
		{"SQLITE_DETACH", 25},
		{"SQLITE_DONE", 101},
		{"SQLITE_DROP_INDEX", 10},
		{"SQLITE_DROP_TABLE", 11},
		{"SQLITE_DROP_TEMP_INDEX", 12},
		{"SQLITE_DROP_TEMP_TABLE", 13},
		{"SQLITE_DROP_TEMP_TRIGGER", 14},
		{"SQLITE_DROP_TEMP_VIEW", 15},
		{"SQLITE_DROP_TRIGGER", 16},
		{"SQLITE_DROP_VIEW", 17},
		{"SQLITE_DROP_VTABLE", 30},
		{"SQLITE_EMPTY", 16},
		{"SQLITE_ERROR", 1},
		{"SQLITE_ERROR_MISSING_COLLSEQ", 257},
		{"SQLITE_ERROR_RETRY", 513},
		{"SQLITE_ERROR_SNAPSHOT", 769},
		{"SQLITE_FORMAT", 24},
		{"SQLITE_FULL", 13},
		{"SQLITE_FUNCTION", 31},
		{"SQLITE_IGNORE", 2},
		{"SQLITE_INSERT", 18},
		{"SQLITE_INTERNAL", 2},
		{"SQLITE_INTERRUPT", 9},
		{"SQLITE_IOERR", 10},
		{"SQLITE_IOERR_ACCESS", 3338},
		{"SQLITE_IOERR_AUTH", 7178},
		{"SQLITE_IOERR_BEGIN_ATOMIC", 7434},
		{"SQLITE_IOERR_BLOCKED", 2826},
		{"SQLITE_IOERR_CHECKRESERVEDLOCK", 3594},
		{"SQLITE_IOERR_CLOSE", 4106},
		{"SQLITE_IOERR_COMMIT_ATOMIC", 7690},
		{"SQLITE_IOERR_CONVPATH", 6666},
		{"SQLITE_IOERR_CORRUPTFS", 8458},
		{"SQLITE_IOERR_DATA", 8202},
		{"SQLITE_IOERR_DELETE", 2570},
		{"SQLITE_IOERR_DELETE_NOENT", 5898},
		{"SQLITE_IOERR_DIR_CLOSE", 4362},
		{"SQLITE_IOERR_DIR_FSYNC", 1290},
		{"SQLITE_IOERR_FSTAT", 1802},
		{"SQLITE_IOERR_FSYNC", 1034},
		{"SQLITE_IOERR_GETTEMPPATH", 6410},
		{"SQLITE_IOERR_LOCK", 3850},
		{"SQLITE_IOERR_MMAP", 6154},
		{"SQLITE_IOERR_NOMEM", 3082},
		{"SQLITE_IOERR_RDLOCK", 2314},
		{"SQLITE_IOERR_READ", 266},
		{"SQLITE_IOERR_ROLLBACK_ATOMIC", 7946},
		{"SQLITE_IOERR_SEEK", 5642},
		{"SQLITE_IOERR_SHMLOCK", 5130},
		{"SQLITE_IOERR_SHMMAP", 5386},
		{"SQLITE_IOERR_SHMOPEN", 4618},
		{"SQLITE_IOERR_SHMSIZE", 4874},
		{"SQLITE_IOERR_SHORT_READ", 522},
		{"SQLITE_IOERR_TRUNCATE", 1546},
		{"SQLITE_IOERR_UNLOCK", 2058},
		{"SQLITE_IOERR_VNODE", 6922},
		{"SQLITE_IOERR_WRITE", 778},
		{"SQLITE_LIMIT_ATTACHED", 7},
		{"SQLITE_LIMIT_COLUMN", 2},
		{"SQLITE_LIMIT_COMPOUND_SELECT", 4},
		{"SQLITE_LIMIT_EXPR_DEPTH", 3},
		{"SQLITE_LIMIT_FUNCTION_ARG", 6},
		{"SQLITE_LIMIT_LENGTH", 0},
		{"SQLITE_LIMIT_LIKE_PATTERN_LENGTH", 8},
		{"SQLITE_LIMIT_SQL_LENGTH", 1},
		{"SQLITE_LIMIT_TRIGGER_DEPTH", 10},
		{"SQLITE_LIMIT_VARIABLE_NUMBER", 9},
		{"SQLITE_LIMIT_VDBE_OP", 5},
		{"SQLITE_LIMIT_WORKER_THREADS", 11},
		{"SQLITE_LOCKED", 6},
		{"SQLITE_LOCKED_SHAREDCACHE", 262},
		{"SQLITE_LOCKED_VTAB", 518},
		{"SQLITE_MISMATCH", 20},
		{"SQLITE_MISUSE", 21},
		{"SQLITE_NOLFS", 22},
		{"SQLITE_NOMEM", 7},
		{"SQLITE_NOTADB", 26},
		{"SQLITE_NOTFOUND", 12},
		{"SQLITE_NOTICE", 27},
		{"SQLITE_NOTICE_RECOVER_ROLLBACK", 539},
		{"SQLITE_NOTICE_RECOVER_WAL", 283},
		{"SQLITE_OK", 0},
		{"SQLITE_OK_LOAD_PERMANENTLY", 256},
		{"SQLITE_OK_SYMLINK", 512},
		{"SQLITE_PERM", 3},
		{"SQLITE_PRAGMA", 19},
		{"SQLITE_PROTOCOL", 15},
		{"SQLITE_RANGE", 25},
		{"SQLITE_READ", 20},
		{"SQLITE_READONLY", 8},
		{"SQLITE_READONLY_CANTINIT", 1288},
		{"SQLITE_READONLY_CANTLOCK", 520},
		{"SQLITE_READONLY_DBMOVED", 1032},
		{"SQLITE_READONLY_DIRECTORY", 1544},
		{"SQLITE_READONLY_RECOVERY", 264},
		{"SQLITE_READONLY_ROLLBACK", 776},
		{"SQLITE_RECURSIVE", 33},
		{"SQLITE_REINDEX", 27},
		{"SQLITE_ROW", 100},
		{"SQLITE_SAVEPOINT", 32},
		{"SQLITE_SCHEMA", 17},
		{"SQLITE_SELECT", 21},
		{"SQLITE_TOOBIG", 18},
		{"SQLITE_TRANSACTION", 22},
		{"SQLITE_UPDATE", 23},
		{"SQLITE_WARNING", 28},
		{"SQLITE_WARNING_AUTOINDEX", 284},
		{"threadsafety", 0},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	// sqlite_version is "0.0.0": no SQLite library is linked, so this build claims
	// no engine version rather than one it cannot honor. dbapi2.py splits it into
	// sqlite_version_info at import, so it must be a dotted-int string.
	if err := objects.StoreAttr(m, "sqlite_version", objects.NewStr("0.0.0")); err != nil {
		return err
	}

	// Row and PrepareProtocol are ordinary types in CPython; keep them as real
	// classes so collections.abc.Sequence.register(Row) at import and any
	// isinstance/subclass use resolve. Connection, Cursor, and Blob are likewise
	// types; they are not instantiated at import (connect() builds a Connection,
	// and it raises), so plain types are faithful to their being types.
	for _, name := range []string{"Row", "PrepareProtocol", "Connection", "Cursor", "Blob"} {
		cls, err := objects.NewClass(name, "sqlite3."+name, nil, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		if err := objects.StoreAttr(m, name, cls); err != nil {
			return err
		}
	}

	// adapters and converters are the registries register_adapter and
	// register_converter populate. They are empty dicts here; nothing reads them
	// before connect() raises.
	for _, name := range []string{"adapters", "converters"} {
		d, err := objects.NewDict(nil, nil)
		if err != nil {
			return err
		}
		if err := objects.StoreAttr(m, name, d); err != nil {
			return err
		}
	}

	// register_adapter and register_converter are called from dbapi2.py at import
	// (through register_adapters_and_converters), so they must not raise; they are
	// inert because no engine consults their registrations. enable_callback_tracebacks
	// is a debug toggle with nothing to trace, so it is a safe no-op too.
	noop := func(name string) error {
		fn := objects.NewFuncKw(name, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			return objects.None, nil
		})
		return objects.StoreAttr(m, name, fn)
	}
	for _, name := range []string{"register_adapter", "register_converter", "enable_callback_tracebacks"} {
		if err := noop(name); err != nil {
			return err
		}
	}

	// connect, adapt, and complete_statement all need the SQLite engine (open a
	// database, adapt a value through the engine's protocol, or ask the C library
	// whether a statement is complete), which this build does not carry, so they
	// raise at the call. dbapi2.py never calls them at import.
	for _, name := range []string{"connect", "adapt", "complete_statement"} {
		fn := objects.NewFuncKw(name, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			return nil, sqlite3Unavailable()
		})
		if err := objects.StoreAttr(m, name, fn); err != nil {
			return err
		}
	}
	return nil
}
