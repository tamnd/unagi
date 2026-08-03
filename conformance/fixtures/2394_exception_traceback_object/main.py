import os
import sys
import traceback


def g():
    raise ValueError("boom")


def f():
    g()


def caller():
    f()


try:
    caller()
except ValueError as e:
    tb = e.__traceback__
    print("tb is None:", tb is None)
    print("type:", type(tb).__name__)
    for fs in traceback.extract_tb(tb):
        print("extract:", fs.name, fs.lineno, fs.line)
    node = tb
    depth = 0
    while node is not None:
        code = node.tb_frame.f_code
        print("walk:", os.path.basename(code.co_filename), code.co_name, code.co_qualname, node.tb_lineno)
        node = node.tb_next
        depth += 1
    print("depth:", depth)
    print("cached identity:", e.__traceback__ is e.__traceback__)
    print("exc_info frames:", len(traceback.extract_tb(sys.exc_info()[2])))

print("fresh:", ValueError("x").__traceback__)
ex = ValueError("y")
print("with_traceback returns self:", ex.with_traceback(None) is ex)
print("override None:", ex.__traceback__)
