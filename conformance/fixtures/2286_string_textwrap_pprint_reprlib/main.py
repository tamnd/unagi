# Four pure text-handling modules. string carries Template, Formatter, capwords
# and the character-class constants; textwrap wraps, fills, shortens, indents
# and dedents text; pprint pretty-prints containers with sorted keys and width
# control; reprlib builds size-limited reprs. None need anything past the floor.
import string
import textwrap
import pprint
import reprlib

# string.Template does $-substitution, strict and safe.
t = string.Template("$name scored $score")
print("substitute:", t.substitute(name="Ann", score=9))
print("safe:", string.Template("$a then $b").safe_substitute(a="one"))
print("braced:", string.Template("${greeting}!").substitute(greeting="hi"))

# capwords and the class constants.
print("capwords:", string.capwords("the quick brown fox"))
print("digits:", string.digits)
print("ascii_lowercase:", string.ascii_lowercase)
print("punctuation set:", "!" in string.punctuation, "a" in string.punctuation)

# string.Formatter drives str.format from the outside.
fmt = string.Formatter()
print("format:", fmt.format("{0} / {name} / {1:>5}", "a", 42, name="b"))
print("vformat:", fmt.format("{x}-{y}", x=1, y=2))

# textwrap wraps to a width, fills to a joined string, and shortens with a
# placeholder when the text overflows.
print("wrap:", textwrap.wrap("the quick brown fox jumps over", width=12))
print("fill:", repr(textwrap.fill("alpha beta gamma delta", width=11)))
print("shorten:", textwrap.shorten("one two three four five", width=16))

# indent adds a prefix to each line, dedent strips common leading whitespace.
print("indent:", repr(textwrap.indent("a\nb\nc\n", "| ")))
print("dedent:", repr(textwrap.dedent("        x\n        y\n")))

# a TextWrapper instance carries the configured options.
w = textwrap.TextWrapper(width=8, initial_indent=">>", subsequent_indent="..")
print("wrapper:", w.wrap("aa bb cc dd"))

# pprint sorts dict keys and breaks onto multiple lines past the width.
pprint.pprint({"gamma": 3, "alpha": 1, "beta": 2})
nested = {"nums": list(range(6)), "pair": {"k": "v"}}
print("pformat:")
print(pprint.pformat(nested, width=20))
print("readable:", pprint.isreadable({"a": [1, 2]}))
print("recursive:", pprint.isrecursive([1, 2, 3]))
print("saferepr:", pprint.saferepr({"z": 1, "a": 2}))

# reprlib limits the size of a repr: long lists and strings get elided.
r = reprlib.Repr()
print("long list:", r.repr(list(range(50))))
print("long str:", reprlib.repr("y" * 60))
print("nested depth:", r.repr([[[[[[1]]]]]]))

# recursive_repr guards a __repr__ that could recurse.
class Node:
    def __init__(self):
        self.link = None
    @reprlib.recursive_repr()
    def __repr__(self):
        return "Node(%r)" % self.link

n = Node()
n.link = n
print("recursive_repr:", repr(n))
