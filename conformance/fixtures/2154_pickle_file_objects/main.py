import io
import pickle
import shelve
from collections import UserDict

# Pickler/Unpickler over a BytesIO round-trip a container graph.
buf = io.BytesIO()
p = pickle.Pickler(buf, 4)
p.dump({'a': 1, 'b': [2, 3], 't': (4, 5), 's': {7, 8, 9}})
data = buf.getvalue()
loaded = pickle.Unpickler(io.BytesIO(data)).load()
print(loaded['a'], loaded['b'], loaded['t'], sorted(loaded['s']))

# The module dump/load functions mirror the classes.
out = io.BytesIO()
pickle.dump(['x', 'y', {'z': 9}], out)
out.seek(0)
print(pickle.load(out))

# clear_memo is accepted and dumping still works afterward.
buf2 = io.BytesIO()
pk = pickle.Pickler(buf2)
pk.clear_memo()
pk.dump((1, 2, 3))
print(pickle.loads(buf2.getvalue()))

# bytes_types is the (bytes, bytearray) pair.
print(pickle.bytes_types)
print(isinstance(b'ab', pickle.bytes_types), isinstance(bytearray(b'x'), pickle.bytes_types), isinstance('ab', pickle.bytes_types))

# A file without a write attribute is rejected at construction.
try:
    pickle.Pickler(object())
except TypeError as e:
    print('write:', e)
try:
    pickle.Unpickler(object())
except TypeError as e:
    print('read:', e)

# shelve.Shelf over an in-memory mapping stores and restores pickled values.
s = shelve.Shelf(UserDict(), protocol=4)
s['k1'] = {'nested': [1, 2, 3]}
s['k2'] = (10, 20)
print(s['k1'], s['k2'])
print(sorted(s.keys()), 'k1' in s, 'zz' in s)
del s['k2']
print(sorted(s.keys()))
