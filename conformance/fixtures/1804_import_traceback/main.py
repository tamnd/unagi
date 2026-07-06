import deep

try:
    deep.blow(1)
except ValueError as e:
    print("caught:", e)

deep.blow(2)
