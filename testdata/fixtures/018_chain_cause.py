def convert():
    try:
        int("x12")
    except ValueError as e:
        raise TypeError("bad input") from e
convert()
