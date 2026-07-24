import csv
import io

d = {'a': 1, 'b': 2, 'c': 3}

# dict view set operators, both against any iterable and against real sets
print("keys-sub-list", sorted(d.keys() - ['a']))
print("keys-sub-set", sorted(d.keys() - {'a'}))
print("keys-and", sorted(d.keys() & {'a', 'z'}))
print("keys-or", sorted(d.keys() | {'z'}))
print("keys-xor", sorted(d.keys() ^ {'a', 'z'}))
print("keys-sub-dict", sorted(d.keys() - d))
print("set-sub-keys", sorted({'a', 'z'} - d.keys()))
print("list-sub-keys", sorted(['a', 'z'] - d.keys()))

# result is always a plain set, even with a frozenset operand
print("type", type(d.keys() - ['a']).__name__)
print("frozen-sub-keys", sorted(frozenset({'a', 'z'}) - d.keys()),
      type(frozenset({'a', 'z'}) - d.keys()).__name__)

# items view is set-like too
print("items-sub", sorted(d.items() - {('a', 1)}))
print("items-and", sorted(d.items() & {('a', 1), ('x', 9)}))

# values view is not set-like
try:
    d.values() - {1}
except TypeError as ex:
    print("values", type(ex).__name__)

# a non-iterable other is a TypeError
try:
    d.keys() - 5
except TypeError as ex:
    print("non-iter", type(ex).__name__)

# max and min with a key callable return the item, first winner on ties
items = [('a', 3), ('b', 1), ('c', 2)]
print("max-key", max(items, key=lambda x: x[1]))
print("min-key", min(items, key=lambda x: x[1]))
print("max-len", max(['a', 'ccc', 'bb'], key=len))
print("min-abs", min([3, -5, 2], key=abs))
print("max-tie", max([(1, 'a'), (1, 'b')], key=lambda x: x[0]))
print("max-default", max([], key=len, default='none'))

# csv.DictWriter and csv.Sniffer now run end to end on the views and key above
out = io.StringIO()
w = csv.DictWriter(out, fieldnames=['a', 'b'])
w.writeheader()
w.writerow({'a': 1, 'b': 2})
w.writerows([{'a': 3, 'b': 4}])
print("dictwriter", repr(out.getvalue()))

out2 = io.StringIO()
w2 = csv.DictWriter(out2, fieldnames=['a'])
try:
    w2.writerow({'a': 1, 'z': 9})
except ValueError as ex:
    print("extra-key", 'z' in str(ex))

sn = csv.Sniffer()
print("sniff-delim", repr(sn.sniff('a;b;c\r\n1;2;3\r\n').delimiter))
print("has-header", sn.has_header('name,age\r\nalice,30\r\nbob,25\r\n'))
