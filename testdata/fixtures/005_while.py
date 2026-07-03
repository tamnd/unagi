n = 5
total = 0
while n > 0:
    total += n
    n -= 1
print(total)
i = 0
while i < 10:
    i += 1
    if i == 3:
        continue
    if i > 6:
        break
    print(i)
else:
    print("no break")
k = 0
while k < 3:
    k += 1
else:
    print("finished", k)
