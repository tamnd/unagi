import _socket as s
print(s.AF_INET, s.SOCK_STREAM, s.SOCK_DGRAM, s.IPPROTO_TCP, s.IPPROTO_UDP, s.IPPROTO_IP)
print(s.htons(1), s.htons(256), s.ntohs(256), s.htonl(1), s.ntohl(16777216))
print(repr(s.inet_aton('1.2.3.4')))
print(s.inet_ntoa(b'\x01\x02\x03\x04'))
print(repr(s.inet_pton(s.AF_INET, '1.2.3.4')))
print(repr(s.inet_pton(s.AF_INET6, '::1')))
print(s.inet_ntop(s.AF_INET, b'\x01\x02\x03\x04'))
print(s.inet_ntop(s.AF_INET6, b'\x00'*15+b'\x01'))
print(s.getdefaulttimeout())
s.setdefaulttimeout(3.5); print(s.getdefaulttimeout()); s.setdefaulttimeout(None); print(s.getdefaulttimeout())
print(s.error is OSError, s.timeout is TimeoutError)
print(issubclass(s.gaierror, OSError), issubclass(s.herror, OSError), s.gaierror.__name__)
print(s.has_ipv6)

def show(fn):
    try:
        fn()
    except Exception as e:
        print(type(e).__name__, str(e))

show(lambda: s.inet_aton('1.2.3.4.5'))
show(lambda: s.inet_ntoa(b'\x01\x02\x03'))
show(lambda: s.htons(-1))
show(lambda: s.htons(70000))
show(lambda: s.htonl(2**33))
show(lambda: s.inet_pton(s.AF_INET, 'bad'))
show(lambda: s.inet_ntop(s.AF_INET, b'\x01\x02'))
