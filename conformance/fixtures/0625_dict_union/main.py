# PEP 584 dict union: binary | builds a new dict, |= merges in place.

# Binary | keeps the left key order, then the right's new keys, right wins on
# an overlap, and both operands must be dicts.
print({1: "a", 2: "b"} | {2: "x", 3: "c"})
print({3: 1, 1: 2} | {2: 3, 1: 9})

left = {1: "a"}
right = {2: "b"}
merged = left | right
print(merged, left, right)

for bad in ([(1, 2)], "ab", 5):
    try:
        {1: 2} | bad
    except TypeError as err:
        print("dict|:", err)

try:
    [(1, 2)] | {1: 2}
except TypeError as err:
    print("list|dict:", err)

# |= merges in place and is alias-visible; it accepts a mapping or an iterable
# of pairs, wider than the binary form.
d = {1: "a"}
alias = d
d |= {2: "b"}
print(d, d is alias)

d = {1: "a"}
d |= [(3, "c"), (1, "z")]
print(d)

d = {1: "a"}
try:
    d |= 5
except TypeError as err:
    print("|=int:", err)

d = {1: "a"}
try:
    d |= [1, 2, 3]
except (TypeError, ValueError) as err:
    print("|=bad:", err)
