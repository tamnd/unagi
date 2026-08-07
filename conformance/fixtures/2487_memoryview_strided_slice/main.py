# A one-dimensional memoryview slice with an extended step is a strided view over
# the same buffer, not a contiguous copy: it reports itself non-contiguous, keeps
# a real stride, still aliases writes back to the source, and a codec that needs a
# flat span such as binascii rejects it with BufferError.
import binascii


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


base = bytearray(b'noncontig')
m = memoryview(base)
rev = m[::-2]

show("c_contiguous", lambda: rev.c_contiguous)
show("contiguous", lambda: rev.contiguous)
show("strides", lambda: rev.strides)
show("shape", lambda: rev.shape)
show("ndim", lambda: rev.ndim)
show("len", lambda: len(rev))
show("tobytes", lambda: rev.tobytes())
show("tolist", lambda: rev.tolist())
show("hex", lambda: rev.hex())
show("first", lambda: rev[0])
show("last", lambda: rev[-1])
show("iterate", lambda: list(rev))
show("readonly", lambda: rev.readonly)

# Slicing a strided view composes over its stride.
show("reslice", lambda: rev[1:3].tobytes())
show("reslice_strides", lambda: rev[1:3].strides)

# toreadonly and re-viewing keep the layout.
show("toreadonly_strides", lambda: rev.toreadonly().strides)
show("toreadonly_bytes", lambda: rev.toreadonly().tobytes())
show("review_strides", lambda: memoryview(rev).strides)

# A positive extended step is strided too, and a single-element slice is contiguous.
show("pos_strides", lambda: m[::2].strides)
show("pos_bytes", lambda: m[::2].tobytes())
show("one_contig", lambda: m[::9].c_contiguous)

# The strided view aliases writes back to the base buffer.
def write_elem():
    w = memoryview(bytearray(b'abcdefghi'))
    s = w[::2]
    s[0] = ord('Z')
    return w.tobytes()


show("write_alias", write_elem)


def write_slice():
    b = bytearray(b'abcdefghi')
    memoryview(b)[::2] = b'01234'
    return bytes(b)


show("write_slice", write_slice)

# binascii needs a C-contiguous buffer, so a strided view is a BufferError, while
# the a2b decoders fail the buffer request as a plain argument TypeError.
show("b2a_hex", lambda: binascii.b2a_hex(rev))
show("hexlify", lambda: binascii.hexlify(rev))
show("crc32", lambda: binascii.crc32(rev))
show("a2b_hex", lambda: binascii.a2b_hex(m[::2]))
