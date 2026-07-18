import threading

i = threading.get_ident()
print(i == threading.get_ident())
print(isinstance(i, int))
print(type(i).__name__)
