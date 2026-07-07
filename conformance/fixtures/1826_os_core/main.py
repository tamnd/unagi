# os at portable-core scope: the process and environment surface a small program
# reaches for, provided in Go behind the os import. Only the stable, cross-host
# facts go in the golden; the actual cwd and environment vary by machine, so the
# fixture checks shapes and a key it sets itself.
import os

print("name:", os.name)
print("sep:", repr(os.sep))
print("linesep:", repr(os.linesep))
print("pathsep:", repr(os.pathsep))
print("curdir:", os.curdir)
print("pardir:", os.pardir)
print("extsep:", os.extsep)
print("altsep:", os.altsep)
print("devnull:", os.devnull)

print("pid positive:", os.getpid() > 0)
print("cwd is str:", isinstance(os.getcwd(), str))
print("cwd absolute:", os.getcwd().startswith("/"))

print("getenv missing:", os.getenv("UNAGI_NOPE_XYZ"))
print("getenv default:", os.getenv("UNAGI_NOPE_XYZ", "fallback"))

os.environ["UNAGI_TEST_KEY"] = "hello"
print("environ get:", os.environ["UNAGI_TEST_KEY"])
print("getenv after set:", os.getenv("UNAGI_TEST_KEY"))
print("in environ:", "UNAGI_TEST_KEY" in os.environ)
print("environ.get default:", os.environ.get("UNAGI_NOPE", "d"))

try:
    os.environ["NO_SUCH_KEY_ABC"]
except KeyError as e:
    print("KeyError:", e)
