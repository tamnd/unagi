# binascii.b2a_qp and binascii.a2b_qp are the quoted-printable codec the quopri
# and email packages build on. They were missing, so this pins the encode and
# decode surface: the =XX escapes, the context-sensitive whitespace and leading
# dot rules, the 76-column soft line breaks, the CRLF mirroring, the header mode
# and a full roundtrip, all against CPython.
import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=> ERR", type(e).__name__, str(e))


show("b2a plain", lambda: binascii.b2a_qp(b"hello world"))
show("b2a latin1", lambda: binascii.b2a_qp(b"caf\xe9 na\xefve"))
show("b2a equals", lambda: binascii.b2a_qp(b"1=1 always"))
show("b2a space before nl", lambda: binascii.b2a_qp(b"trail   \nnext"))
show("b2a tab before nl", lambda: binascii.b2a_qp(b"tab\t\nnext"))
show("b2a leading dot", lambda: binascii.b2a_qp(b".\n.line\nmid.dot"))
show("b2a quotetabs", lambda: binascii.b2a_qp(b"a\tb c\td", quotetabs=True))
show("b2a header spaces", lambda: binascii.b2a_qp(b"key = value here", header=True))
show("b2a istext false", lambda: binascii.b2a_qp(b"a\r\nb\nc", istext=False))
show("b2a controls", lambda: binascii.b2a_qp(b"x\x00\x01\x07\x1f y"))
show("b2a high range", lambda: binascii.b2a_qp(bytes(range(120, 136))))
show("b2a long line wrap", lambda: binascii.b2a_qp(b"a" * 80))
show("b2a long then nl", lambda: binascii.b2a_qp(b"z" * 100 + b"\ntail"))
show("b2a crlf input", lambda: binascii.b2a_qp(b"one\r\ntwo\r\nthree"))
show("b2a positional", lambda: binascii.b2a_qp(b"a\tb c", True, True, False))
show("b2a empty", lambda: binascii.b2a_qp(b""))

show("a2b plain", lambda: binascii.a2b_qp(b"hello=20world"))
show("a2b latin1", lambda: binascii.a2b_qp(b"caf=E9"))
show("a2b lower hex", lambda: binascii.a2b_qp(b"=e9=ff"))
show("a2b soft lf", lambda: binascii.a2b_qp(b"joined=\nup"))
show("a2b soft crlf", lambda: binascii.a2b_qp(b"joined=\r\nup"))
show("a2b escaped eq", lambda: binascii.a2b_qp(b"a==3Db"))
show("a2b header underscore", lambda: binascii.a2b_qp(b"a_b_c", header=True))
show("a2b bad escape kept", lambda: binascii.a2b_qp(b"a=Zb=Gh"))
show("a2b trailing eq", lambda: binascii.a2b_qp(b"tail="))
show("a2b single hex tail", lambda: binascii.a2b_qp(b"ab=E"))

# A roundtrip through both directions returns the original bytes.
payload = b"caf\xe9 with  spaces\tand\x00nulls and a much longer stretch " + b"y" * 90
show("roundtrip", lambda: binascii.a2b_qp(binascii.b2a_qp(payload)) == payload)

show("b2a bad keyword", lambda: binascii.b2a_qp(b"x", nope=1))
show("a2b bad keyword", lambda: binascii.a2b_qp(b"x", nope=1))
