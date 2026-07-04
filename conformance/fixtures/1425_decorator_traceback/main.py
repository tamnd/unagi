def boom(f):
    raise ValueError("decorator failed")


print("before")


@boom
def target():
    pass


print("after")
