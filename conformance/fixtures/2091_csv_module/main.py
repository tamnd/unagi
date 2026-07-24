import csv
import io


def show(label, fn):
    try:
        print(label, fn())
    except (csv.Error, TypeError, ValueError) as ex:
        print(label, type(ex).__name__ + ':', ex)


for q, name in [(csv.QUOTE_MINIMAL, 'min'), (csv.QUOTE_ALL, 'all'),
                (csv.QUOTE_NONNUMERIC, 'nn'), (csv.QUOTE_NONE, 'none')]:
    out = io.StringIO()
    w = csv.writer(out, quoting=q, escapechar='\\')
    w.writerow(['a', 'b,c', 'd"e', 1, 2.5])
    print(name, repr(out.getvalue()))

out = io.StringIO()
w = csv.writer(out, doublequote=False, escapechar='\\')
w.writerow(['he said "hi"'])
print("esc", repr(out.getvalue()))

data = 'a,b,c\r\n1,"x,y",3\r\n"line1\nline2",p,q\r\n'
print("read", list(csv.reader(io.StringIO(data))))
print("skip", list(csv.reader(['a,  b,   c'], skipinitialspace=True)))
print("nnread", list(csv.reader(['1,2,3'], quoting=csv.QUOTE_NONNUMERIC)))

print("dialects", sorted(csv.list_dialects()))
csv.register_dialect('pipe', delimiter='|')
print("pipe", list(csv.reader(['a|b|c'], dialect='pipe')))
print("has-pipe", 'pipe' in csv.list_dialects())
csv.unregister_dialect('pipe')
print("no-pipe", 'pipe' in csv.list_dialects())

old = csv.field_size_limit()
print("fsl", old, csv.field_size_limit(1000))
csv.field_size_limit(old)

dr = csv.DictReader(io.StringIO('name,age\r\nalice,30\r\nbob,25\r\n'))
print("dictread", [dict(r) for r in dr])

out = io.StringIO()
w = csv.writer(out)
w.writerows([['x', 'y'], ['1', '2']])
print("rt", list(csv.reader(io.StringIO(out.getvalue()))))

show("bad-delim", lambda: csv.reader([], delimiter='xx'))
show("no-escape", lambda: (lambda o: (csv.writer(o, quoting=csv.QUOTE_NONE).writerow(['a,b']), o.getvalue()))(io.StringIO()))
