import pyexpat
import xml.parsers.expat as expat
import plistlib
import xml.dom.minidom as minidom

# Module identity and the errors/model submodules the shim republishes. The
# bundled libexpat version drifts between hosts, so only the "expat_" prefix of
# EXPAT_VERSION is asserted rather than the exact release.
print(expat.EXPAT_VERSION.startswith("expat_"))
print(expat.native_encoding)
print(expat.error is expat.ExpatError)
print(expat.errors.XML_ERROR_SYNTAX)
print(expat.errors.codes["syntax error"])
print(expat.model.XML_CTYPE_EMPTY, expat.model.XML_CQUANT_PLUS)
print(expat.ErrorString(2))

# Direct parse: the event stream a caller sees, covering the XML declaration,
# a comment, a processing instruction, attributes, text and nesting.
events = []
p = expat.ParserCreate()
p.XmlDeclHandler = lambda v, e, s: events.append(("xmldecl", v, e, s))
p.CommentHandler = lambda d: events.append(("comment", d))
p.ProcessingInstructionHandler = lambda t, d: events.append(("pi", t, d))
p.StartElementHandler = lambda n, a: events.append(("start", n, a))
p.EndElementHandler = lambda n: events.append(("end", n))
p.CharacterDataHandler = lambda d: events.append(("chars", d))
p.Parse('<?xml version="1.0" encoding="UTF-8"?><!-- c --><r a="1"><?go now?>hi<b/>bye</r>', True)
for e in events:
    print(e)

# Namespace mode: element and attribute names carry the resolved URI, and the
# namespace declarations are reported.
nsevents = []
np = expat.ParserCreate(None, " ")
np.StartNamespaceDeclHandler = lambda pre, uri: nsevents.append(("ns", pre, uri))
np.StartElementHandler = lambda n, a: nsevents.append(("start", n, a))
np.EndElementHandler = lambda n: nsevents.append(("end", n))
np.Parse('<r xmlns:p="urn:x" xmlns="urn:d"><p:c a="1">t</p:c></r>', True)
for e in nsevents:
    print(e)

# buffer_text coalesces adjacent character data into one callback.
buf = []
bp = expat.ParserCreate()
bp.buffer_text = True
bp.CharacterDataHandler = lambda d: buf.append(d)
bp.StartElementHandler = lambda n, a: None
bp.EndElementHandler = lambda n: None
bp.Parse('<r>ab<x/>cd</r>', True)
print(buf)

# plistlib drives expat through ParseFile: a full round-trip must be identical.
data = {"name": "unagi", "n": 42, "ok": True, "items": [1, 2, 3], "nested": {"x": 1.5}}
blob = plistlib.dumps(data)
print(plistlib.loads(blob) == data)
print(blob.decode())

# minidom drives expat in namespace mode; the reserialized document must match.
doc = minidom.parseString('<a x="1"><b>text</b><c/></a>')
print(doc.documentElement.tagName, doc.documentElement.getAttribute("x"))
print(doc.toxml())
