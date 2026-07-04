def inner():
    raise ValueError("deep failure")
def outer():
    inner()
print("before the crash")
outer()
