d = {"a": 1, "b": 2}
print(d)
d["c"] = 3
print(d)
print(d["a"])
print(len(d))
print("b" in d)
print("z" in d)
print(d.get("a"))
print(d.get("z"))
print(d.get("z", 0))
print(d.keys())
print(d.values())
print(d.items())
for k in d:
    print(k, d[k])
for kv in d.items():
    print(kv)
print(d.pop("a"))
print(d)
print(d.pop("zz", -1))
d2 = {1: "one", 2.5: "two-five", True: "yes"}
print(d2[1.0])
print(d2)
