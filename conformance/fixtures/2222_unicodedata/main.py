import unicodedata as u

# category() is computed for real from the same Unicode tables, so it matches
# CPython character for character across every general category.
samples = ['A', 'z', 'ǅ', 'ʰ', '中', 'é', '́', 'ः', '⃝', '7', 'Ⅻ', '½',
           '_', '-', '(', ')', '«', '»', '!', '+', '$', '^', '©',
           ' ', '\x00', '​']
print(' '.join(u.category(c) for c in samples))

# ASCII digit values.
print(u.decimal('9'), u.digit('4'), u.numeric('6'))

# decimal/digit/numeric fall to a supplied default for a non-digit.
print(u.decimal('a', -1), u.digit('z', -2), u.numeric('m', -3.0))

# With no default a non-digit raises ValueError.
for fn in (u.decimal, u.digit, u.numeric):
    try:
        fn('a')
    except ValueError:
        print('ValueError')

# east_asian_width over the wide, fullwidth, and narrow-ASCII blocks.
print(u.east_asian_width('中'), u.east_asian_width('Ａ'), u.east_asian_width('A'))

# ASCII is unchanged and already normalized in every form.
print(u.normalize('NFC', 'hello'), u.normalize('NFKD', 'world'))
print(u.is_normalized('NFC', 'hello'), u.is_normalized('NFKC', 'plain'))

# A bad normalization form is a ValueError.
try:
    u.normalize('NFX', 'x')
except ValueError:
    print('bad form')

# combining is 0 for a plain letter; decomposition is empty for a letter.
print(u.combining('a'), repr(u.decomposition('A')))

# A multi-character argument is a TypeError.
try:
    u.category('ab')
except TypeError:
    print('TypeError')
