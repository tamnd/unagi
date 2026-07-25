import urllib.request as u
from urllib.parse import urlsplit, urljoin, parse_qs

req = u.Request("http://Example.com/path?q=1", headers={"X-Test": "1"})
print(req.get_full_url())
print(req.host)
print(req.get_header("X-test"))
print(req.type)

sp = urlsplit("http://user@host:8080/a/b?x=1&y=2#frag")
print(sp.scheme, sp.hostname, sp.port, sp.path)
print(sp.query, sp.fragment)
print(urljoin("http://a.com/x/y", "../z"))
print(sorted(parse_qs("x=1&y=2&x=3").items()))
print(isinstance(u.getproxies(), dict))
