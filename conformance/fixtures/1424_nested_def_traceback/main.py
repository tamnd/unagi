def outer():
    def inner():
        raise ValueError("boom")
    inner()


outer()
