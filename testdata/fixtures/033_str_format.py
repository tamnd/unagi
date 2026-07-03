print("{} and {}".format(1, "two"))
print("{1} before {0}".format("a", "b"))
print("{0} {0} {1}".format("x", "y"))
print("{!r} {!s}".format("q", "q"))
print("{:>6} {:<6} {:^6}".format("ab", "cd", "ef"), "|")
print("{:08.3f} {:+d} {:x} {:#o} {:,}".format(3.14159, 42, 255, 8, 1234567))
print("{{literal}} {}".format(5))
print("%s %r" % ("hi", "hi"))
print("%d %5d %-5d| %05d" % (7, 7, 7, 7))
print("%+d % d %d" % (3, 3, -3))
print("%x %X %#x %o %#o" % (255, 255, 255, 8, 8))
print("%f %.2f %10.2f %e %E %g" % (2.5, 2.675, 2.675, 12345.678, 12345.678, 100000.0))
print("%g %g" % (1e17, 0.0001))
print("%c %c" % (65, "z"))
print("%s" % [1, 2], "%s" % (3,))
print("%*d|%-*d|%.*f" % (6, 42, 6, 42, 2, 2.675))
print("%(name)s is %(age)d" % {"name": "ana", "age": 3})
print("100%% -> %d%%" % 99)
print(format(3.14159, ".2f"), format(42), format("hi", ">4") + "|", format(255, "#x"))
print(format(True), format(None, ""))
try:
    format(1, 2)
except TypeError as e:
    print("caught", e)
try:
    "{} {}".format(1)
except IndexError as e:
    print("caught", e)
try:
    "{} {0}".format(1)
except ValueError as e:
    print("caught", e)
try:
    "%d %d" % (1,)
except TypeError as e:
    print("caught", e)
try:
    "%d" % (1, 2)
except TypeError as e:
    print("caught", e)
try:
    "%d" % "x"
except TypeError as e:
    print("caught", e)
try:
    "%y" % 1
except ValueError as e:
    print("caught", e)
