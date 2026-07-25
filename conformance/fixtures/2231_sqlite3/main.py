# sqlite3 imports through the reduced-surface _sqlite3 accelerator: its
# constants, DB-API exception hierarchy, and parse flags are all real, so the
# module is usable for the constant and exception surface even though this build
# carries no SQLite engine to open a database with.
import sqlite3

print("name", sqlite3.__name__)
print("apilevel", sqlite3.apilevel)
print("paramstyle", sqlite3.paramstyle)

# The SQLITE_* codes come straight from SQLite's C headers, so they are fixed.
names = [
    "SQLITE_OK", "SQLITE_ERROR", "SQLITE_BUSY", "SQLITE_LOCKED",
    "SQLITE_ROW", "SQLITE_DONE", "SQLITE_CONSTRAINT", "SQLITE_MISUSE",
    "SQLITE_SELECT", "SQLITE_INSERT", "SQLITE_UPDATE", "SQLITE_DELETE",
    "SQLITE_CREATE_TABLE", "SQLITE_DROP_TABLE",
    "SQLITE_LIMIT_LENGTH", "SQLITE_LIMIT_VARIABLE_NUMBER",
    "SQLITE_DBCONFIG_ENABLE_FKEY",
    "PARSE_DECLTYPES", "PARSE_COLNAMES",
]
for n in names:
    print(n, getattr(sqlite3, n))

# The DB-API 2.0 exception hierarchy chains as it does in CPython.
print("Error<-Exception", issubclass(sqlite3.Error, Exception))
print("Warning<-Exception", issubclass(sqlite3.Warning, Exception))
print("DatabaseError<-Error", issubclass(sqlite3.DatabaseError, sqlite3.Error))
print("OperationalError<-DatabaseError",
      issubclass(sqlite3.OperationalError, sqlite3.DatabaseError))
print("IntegrityError<-DatabaseError",
      issubclass(sqlite3.IntegrityError, sqlite3.DatabaseError))
print("NotSupportedError<-DatabaseError",
      issubclass(sqlite3.NotSupportedError, sqlite3.DatabaseError))

# Row and PrepareProtocol are types, and Binary is memoryview via dbapi2.
print("Row is type", isinstance(sqlite3.Row, type))
print("PrepareProtocol is type", isinstance(sqlite3.PrepareProtocol, type))
print("Binary", sqlite3.Binary is memoryview)

# register_adapter is called by dbapi2 at import and is a plain no-op returning
# None, so it is safe to call again.
print("register_adapter", sqlite3.register_adapter(int, str))
