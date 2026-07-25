import lzma
import _lzma

# lzma.py reads only the constant block and the codec constructors at import; it
# builds a compressor lazily inside its helpers, so the surface alone lets the
# module load.
print("imported")
print(lzma.__name__)

# The constant block liblzma exports is real and usable: check ids, filter ids,
# container formats, match finders, encoder modes, and presets.
print(_lzma.CHECK_NONE, _lzma.CHECK_CRC32, _lzma.CHECK_CRC64, _lzma.CHECK_SHA256)
print(_lzma.CHECK_ID_MAX, _lzma.CHECK_UNKNOWN)
print(_lzma.FILTER_LZMA1, _lzma.FILTER_LZMA2, _lzma.FILTER_DELTA)
print(_lzma.FILTER_X86, _lzma.FILTER_ARM, _lzma.FILTER_SPARC)
print(_lzma.FORMAT_AUTO, _lzma.FORMAT_XZ, _lzma.FORMAT_ALONE, _lzma.FORMAT_RAW)
print(_lzma.MF_HC3, _lzma.MF_HC4, _lzma.MF_BT2, _lzma.MF_BT3, _lzma.MF_BT4)
print(_lzma.MODE_FAST, _lzma.MODE_NORMAL)
print(_lzma.PRESET_DEFAULT, _lzma.PRESET_EXTREME)

# lzma re-exports the constants under its own namespace too (from _lzma import *).
print(lzma.FORMAT_XZ, lzma.CHECK_CRC64, lzma.PRESET_DEFAULT)

# LZMAError is a real, catchable Exception subclass.
print(issubclass(lzma.LZMAError, Exception))
print(lzma.LZMAError.__name__)

# The codec itself needs an xz backend the AOT build does not carry, so
# LZMACompressor/LZMADecompressor and the one-shot helpers raise; that is
# exercised in the unit test rather than here where CPython compresses for real.
