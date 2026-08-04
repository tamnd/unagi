import warnings
warnings.filterwarnings("ignore", category=DeprecationWarning)
import array
import copy

# array.__reduce_ex__ is the reduction pickle and copy read to serialize an
# array. Below protocol 3 it reduces to the array type applied to the type code
# and the element list; from protocol 3 up it reduces to _array_reconstructor
# applied to the array type, the type code, the machine format code and the raw
# bytes so a cross-platform pickle records the exact byte layout. The reduction
# callable is printed by identity rather than repr, since unagi reprs a C
# accelerator function as <function ...> where CPython reprs it as
# <built-in function ...>, a separate module-wide repr gap.
cases = [
    ('b', [-1, 2]), ('B', [1, 255]),
    ('h', [-2, 300]), ('H', [1, 65535]),
    ('i', [1, 2, 3]), ('I', [1, 4000000000]),
    ('l', [10, 20]), ('L', [1, 2]),
    ('q', [-5, 5]), ('Q', [7, 8]),
    ('f', [1.5, -2.0]), ('d', [3.25]),
    ('u', 'abc'), ('w', 'xy'),
]

for tc, init in cases:
    a = array.array(tc, init)
    # Below protocol 3: the type applied to (typecode, element list). The type
    # reprs the same in both, so the whole tuple prints byte for byte.
    print("P0", tc, a.__reduce_ex__(0))
    print("P2", tc, a.__reduce_ex__(2))
    # Protocol 3 and up: the reconstructor applied to the machine-format form.
    r = a.__reduce_ex__(5)
    print("P5", tc, r[0] is array._array_reconstructor, r[1], r[2])
    # copy and deepcopy both drive through __reduce_ex__ and rebuild an equal
    # array carrying the same type code.
    c = copy.copy(a)
    d = copy.deepcopy(a)
    print("copy", tc, c == a, c.typecode, d == a, d.typecode)

# The error surface: arity and a non-integer protocol, matched to CPython's C
# clinic wording.
a = array.array('i', [1, 2, 3])
def show(*args):
    try:
        return a.__reduce_ex__(*args)
    except Exception as ex:
        return type(ex).__name__ + ": " + str(ex)

print("no arg", show())
print("two args", show(1, 2))
print("str", show("x"))
print("none", show(None))
# A bool is a valid integer protocol, so True reduces below protocol 3.
print("bool", show(True))
