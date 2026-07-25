import io
args = ["a", "b", "c"]

# star positional + keyword
print(*args, sep="-")

# leading positional, then star, then keyword
print("head", *args, sep="|")

# ** unpacking of keyword options onto a builtin
opts = {"sep": ", ", "end": ";\n"}
print(*args, **opts)

# star positional written to an explicit file object
buf = io.StringIO()
print(*args, sep="/", file=buf)
print("buffer:", repr(buf.getvalue()))

# star with no keywords still works
print(*args)

# unexpected keyword raises TypeError, catchable
try:
    print(*args, bogus=1)
except TypeError as e:
    print("TypeError:", e)
