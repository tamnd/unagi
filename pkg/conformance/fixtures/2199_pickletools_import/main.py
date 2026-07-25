import pickle
import pickletools
import io


# pickletools imports only if its module-body consistency check passes, which
# reads pickle.__all__ and cross-checks every one-byte opcode against its own
# table. So a clean import already exercises the opcode alphabet.
print("MARK" in pickle.__all__, pickle.STOP == b".", pickle.EMPTY_LIST == b"]")
print(pickle.FALSE, pickle.TRUE)


# dis renders a real pickle through that table; the header opcodes are stable
# across runs so the disassembly is a golden.
data = pickle.dumps([1, 2, {"a": 3}], protocol=2)
out = io.StringIO()
pickletools.dis(data, out)
text = out.getvalue()
for op in ("PROTO", "EMPTY_LIST", "EMPTY_DICT", "SETITEM", "STOP"):
    print(op, op in text)


# The encode_long/decode_long codec pickletools pairs with the LONG opcodes.
print(pickletools.read_decimalnl_short is not None)
print(pickle.decode_long(pickle.encode_long(1 << 70)) == 1 << 70)
