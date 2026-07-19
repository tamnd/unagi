# print(file=...) writes through the object's write() method, honouring sep,
# end and flush; a file of None still goes to stdout. Wordings probed on 3.14.
import io

buf = io.StringIO()
print("hello", "world", file=buf)
print(1, 2, 3, sep="-", end="!", file=buf)
print("flushed", file=buf, flush=True)
print(repr(buf.getvalue()))

# None file is stdout, the same as omitting the keyword.
print("to stdout", file=None)
print("plain")

# A user object with write() receives the composed text; a truthy flush calls
# flush() after the write.
class Sink:
    def __init__(self):
        self.parts = []
        self.flushed = 0

    def write(self, s):
        self.parts.append(s)
        return len(s)

    def flush(self):
        self.flushed += 1

s = Sink()
print("a", "b", sep=",", file=s)
print("c", file=s, flush=True)
print(s.parts, s.flushed)

# sep/end type errors still fire on the file path.
try:
    print("x", sep=0, file=buf)
except TypeError as e:
    print(e)
