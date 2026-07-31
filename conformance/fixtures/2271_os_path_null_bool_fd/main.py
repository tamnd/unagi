import os
import warnings


# An embedded NUL in a path is screened before the syscall: os.stat, os.lstat and
# os.access each raise ValueError naming the call, for a str path and a bytes one.
def null_case(label, fn):
    try:
        fn()
    except ValueError as e:
        print(label, "ValueError", str(e))
    except Exception as e:  # pragma: no cover - would be a regression
        print(label, "OTHER", type(e).__name__, str(e))
    else:
        print(label, "no-error")


null_case("stat/str", lambda: os.stat("a\x00b"))
null_case("stat/bytes", lambda: os.stat(b"a\x00b"))
null_case("lstat/str", lambda: os.lstat("a\x00b"))
null_case("lstat/bytes", lambda: os.lstat(b"a\x00b"))
null_case("access/str", lambda: os.access("a\x00b", os.F_OK))
null_case("access/bytes", lambda: os.access(b"a\x00b", os.F_OK))

# A bool handed to the file-descriptor form of os.stat still works — True is fd 1 —
# but emits a RuntimeWarning flagging the near-certain mistake.
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    st = os.stat(True)
    print("stat(True) ok:", st.st_mode >= 0)
    print("warnings:", [(w.category.__name__, str(w.message)) for w in caught])
