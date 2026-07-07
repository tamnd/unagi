# A builtin type object answers __dict__ as a read-only mappingproxy over its
# own namespace, the namespace enum's _find_data_type_ probes for __new__.

print(type(int.__dict__).__name__)

# A constructor type defines __new__, so the name is a member of its namespace,
# and no builtin carries a dataclass fields marker.
for t in (int, str, bool, list, dict, tuple):
    print(t.__name__, "__new__" in t.__dict__, "__dataclass_fields__" in t.__dict__)

# The proxy is read-only.
try:
    int.__dict__["spam"] = 1
except TypeError as e:
    print("TypeError", "does not support item assignment" in str(e))
