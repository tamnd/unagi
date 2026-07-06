def blow(n):
    if n == 0:
        raise ValueError("kaboom")
    return blow(n - 1)
