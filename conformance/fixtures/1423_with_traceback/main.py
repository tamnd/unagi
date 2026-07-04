# A with whose __exit__ raises replaces the body exception, chaining the
# original in as its context, and the report cites the with line for the
# replacement frame.
class CM:
    def __enter__(self):
        return self

    def __exit__(self, et, ev, tb):
        raise TypeError("from exit")


with CM():
    raise ValueError("boom")
