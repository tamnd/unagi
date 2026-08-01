# The remaining pure S1 leaf modules. graphlib is the topological sorter,
# colorsys the colour-space conversions, and the rest are small pure helpers:
# __future__ (feature flags), keyword (the reserved-word tables), _compat_pickle
# (the 2-to-3 name maps), _markupbase (the SGML parser base) and this (the Zen).
import graphlib
import colorsys

# static_order returns one full topological ordering; a dependency comes before
# the node that needs it.
ts = graphlib.TopologicalSorter()
ts.add("a", "b", "c")
ts.add("b", "c")
ts.add("d", "a")
print("static order:", list(ts.static_order()))

# prepare raises CycleError when the graph is not a DAG.
cyclic = graphlib.TopologicalSorter({"x": {"y"}, "y": {"x"}})
try:
    cyclic.prepare()
except graphlib.CycleError as e:
    print("cycle:", e.args[0])

# The step API hands back each ready batch; done() unlocks the dependents.
ts2 = graphlib.TopologicalSorter({0: {1, 2}, 1: {2}, 2: set()})
ts2.prepare()
batches = []
while ts2.is_active():
    ready = ts2.get_ready()
    batches.append(sorted(ready))
    for n in ready:
        ts2.done(n)
print("batches:", batches)

# colorsys round-trips between colour spaces; hls->rgb inverts rgb->hls.
hls = colorsys.rgb_to_hls(0.2, 0.4, 0.6)
print("rgb_to_hls:", hls)
print("hls_to_rgb:", colorsys.hls_to_rgb(*hls))
print("rgb_to_hsv:", colorsys.rgb_to_hsv(0.2, 0.4, 0.6))
print("hsv_to_rgb:", colorsys.hsv_to_rgb(*colorsys.rgb_to_hsv(0.2, 0.4, 0.6)))
print("rgb_to_yiq:", colorsys.rgb_to_yiq(0.2, 0.4, 0.6))
print("yiq_to_rgb:", colorsys.yiq_to_rgb(*colorsys.rgb_to_yiq(0.2, 0.4, 0.6)))

# __future__ carries a feature flag for every named future feature.
import __future__
print("future annotations:", __future__.annotations.optional)
print("future has division:", "division" in __future__.all_feature_names)

# keyword knows the reserved words and the soft keywords.
import keyword
print("iskeyword:", keyword.iskeyword("for"), keyword.iskeyword("spam"))
print("issoftkeyword:", keyword.issoftkeyword("match"), keyword.issoftkeyword("if"))

# _compat_pickle maps the renamed 2-to-3 modules and globals.
import _compat_pickle
print("compat import map:", _compat_pickle.IMPORT_MAPPING["copy_reg"])
print("compat name map:", _compat_pickle.NAME_MAPPING[("__builtin__", "xrange")])

# _markupbase provides the SGML/markup parser base class.
import _markupbase
print("markupbase base:", _markupbase.ParserBase.__name__)

# this prints the Zen on import and exposes the encoded text and its cipher.
import this
print("zen head:", this.s[:13])
print("zen keys:", len(this.d))
