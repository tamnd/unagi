s = "  Hello, World  "
print(repr(s.strip()), repr(s.lstrip()), repr(s.rstrip()))
print(repr("xxhixx".strip("x")), repr("xxhixx".lstrip("x")), repr("xxhixx".rstrip("x")))
print("hello world".capitalize(), "hello world".title(), "HeLLo".swapcase())
print("a b  c".split(), "a,b,,c".split(","), "a b  c".split(None, 1))
print("a,b,c".split(",", 1), "a,b,c".rsplit(",", 1), "a b  c".rsplit(None, 1))
print("one\ntwo\r\nthree".splitlines(), "one\ntwo\n".splitlines(True))
print("abcabc".find("bc"), "abcabc".rfind("bc"), "abcabc".find("bc", 2), "abcabc".find("x"))
print("abcabc".index("bc"), "abcabc".rindex("bc"), "abcabc".count("bc"), "aaa".count("aa"))
print("banana".count("a", 2), "banana".count("a", 2, 4))
print("ab".startswith(("x", "a")), "ab".endswith(("b", "y")), "abc".startswith("b", 1))
print("hello".replace("l", "L", 1), "hello".replace("l", "L"))
print("hi".center(6), "hi".center(6, "*"), "hi".ljust(5, "."), "hi".rjust(5, "."))
print("42".zfill(5), "-42".zfill(5), "+42".zfill(5), "ab".zfill(4))
print("a\tb".expandtabs(), "a\tb".expandtabs(4))
print("key=value=x".partition("="), "key=value=x".rpartition("="))
print("missing".partition(":"), "missing".rpartition(":"))
print("unagi.py".removeprefix("unagi"), "unagi.py".removesuffix(".py"), "abc".removeprefix("x"))
print("abc".isalpha(), "abc1".isalpha(), "abc1".isalnum(), "".isalnum())
print("123".isdigit(), "12.3".isdigit(), "123".isdecimal(), "123".isnumeric())
print("abc".islower(), "Abc".islower(), "ABC".isupper(), "A1".isupper())
print(" \t\n".isspace(), "".isspace(), "abc".isascii(), "café".isascii())
print("Hello World".istitle(), "Hello world".istitle(), "name1".isidentifier(), "1name".isidentifier())
print("abc".isprintable(), "a\tb".isprintable())
try:
    "abc".index("x")
except ValueError as e:
    print("caught", e)
try:
    "hi".center(6, "ab")
except TypeError as e:
    print("caught", e)
try:
    "a,b".split("")
except ValueError as e:
    print("caught", e)
