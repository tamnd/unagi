def add(a, b):
    return a + b

def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)

def greet(name):
    print("hi", name)

def nop():
    pass

def classify(n):
    if n < 0:
        return "negative"
    if n == 0:
        return "zero"
    return "positive"

print(add(2, 3))
print(add("con", "cat"))
print(fib(10))
greet("unagi")
print(nop())
print(classify(-4), classify(0), classify(9))
