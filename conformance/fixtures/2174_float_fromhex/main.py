print(float.fromhex('0x1.8p0'))
print(float.fromhex('0x1p4'))
print(float.fromhex(' 0x1.8p0 '))
print(float.fromhex('-0x1.8p0'))
print(float.fromhex('inf'), float.fromhex('-inf'))
print(float.fromhex('Infinity'))
print(float.fromhex('1.5'))
print(float.fromhex('ff'))
print(float.fromhex('0x1.921fb54442d18p+1'))
print(float.fromhex('0x0p0'), float.fromhex('-0x0p0'))
print(float.fromhex((1.5).hex()) == 1.5)
print(float.fromhex((3.141592653589793).hex()) == 3.141592653589793)
for bad in ['xyz', '0x1.8pq', '', '0x', '0xg']:
    try:
        float.fromhex(bad)
    except ValueError as e:
        print('ValueError:', e)
try:
    float.fromhex('0x1p10000')
except OverflowError as e:
    print('OverflowError:', e)
