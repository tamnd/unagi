class MyStr(str):
    pass


a = MyStr("hello")
b = MyStr("world")

# Concatenation and repetition return a plain str; the subclass does not
# propagate.
print(a + " " + b, type(a + b).__name__)
print(a * 2, 2 * a)

# Concatenation with a plain str on either side.
print("say " + a, a + "!")

# Comparison against plain strings and other instances, both directions.
print(a == "hello", "hello" == a, a < "z", a > b, a != "hello")

# isinstance, issubclass and the class identity.
print(isinstance(a, str), isinstance(a, MyStr), issubclass(MyStr, str), type(a).__name__)

# str, repr, f-string and format specs read through to the underlying str.
print(str(a), repr(a), f"{a}", f"{a!r}", format(a, ">8"))

# Percent formatting treats the instance as its str.
print("%s/%s" % (a, "z"))

# Hashing matches the underlying str, so instances key like their value.
print(hash(a) == hash("hello"))
d = {MyStr("k"): "v"}
print(d["k"])
d2 = {}
d2[a] = 1
d2["hello"] = 2
print(len(d2), d2["hello"], d2[MyStr("hello")])

# len, membership, indexing, slicing and iteration.
print(len(a), "ell" in a, MyStr("ell") in a, a[0], a[1:3], list(a))

# Inherited str methods run on the payload and return plain str.
print(a.upper(), a.capitalize(), a.replace("l", "L"), "-".join([a, b]))
print(a.startswith(MyStr("he")), a.split("l"), type(a.upper()).__name__)

# Truthiness follows the string.
print(bool(MyStr("")), bool(a))

# Sets collapse equal plain and subclass strings.
print(len({a, "hello", MyStr("hello")}))


# A subclass that transforms its value in __new__ ends up with the right str.
class Up(str):
    def __new__(cls, value):
        return super().__new__(cls, value.upper())


u = Up("abc")
print(u, u + "!", type(u).__name__)


# A subclass that overrides an operator keeps its override over the payload.
class Shout(str):
    def __add__(self, other):
        return Shout(str(self) + str(other) + "!")


s = Shout("hi")
print(str(s + "there"), type(s + "there").__name__)
