"""Generate the Linux PDF font instances with FontTools 4.58.5."""

import hashlib
import sys
from pathlib import Path

from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont


SC_SHA256 = "a3041811a78c361b1de50f953c805e0244951c21c5bd412f7232ef0d899af0da"
ITALIC_SHA256 = "58e6e0ebd1931b29a365aa2d3e2ee9a9e831a3af7cf3ad1462d4e72154f0b291"


def derive(source, out, digest, axes, name, slanted):
    if hashlib.sha256(source.read_bytes()).hexdigest() != digest:
        raise ValueError(f"Unexpected source hash: {source}")
    with TTFont(source, recalcTimestamp=False) as font:
        mapping = font.getBestCmap()
        metrics = font["hmtx"].metrics.copy()
        defaults = {axis.axisTag: axis.defaultValue for axis in font["fvar"].axes}
        if set(axes) != set(defaults):
            raise ValueError(f"Not all axes are fixed in {name}")
        instantiateVariableFont(font, axes, inplace=True, updateFontNames=True)
        if "fvar" in font or font["OS/2"].usWeightClass != axes["wght"]:
            raise ValueError(f"Not a static face at the requested weight: {name}")
        if bool(font["post"].italicAngle) != slanted:
            raise ValueError(f"Unexpected italic angle: {name}")
        if font.getBestCmap() != mapping or axes == defaults and font["hmtx"].metrics != metrics:
            raise ValueError(f"Coverage or default metrics changed: {name}")
        font.save(out / name)


if __name__ == "__main__":
    sc, italic, destination = map(Path, sys.argv[1:])
    derive(sc, destination, SC_SHA256, {"wght": 100}, "NotoSansSC.ttf", False)
    derive(sc, destination, SC_SHA256, {"wght": 700}, "NotoSansSC-Bold.ttf", False)
    derive(italic, destination, ITALIC_SHA256, {"wdth": 100, "wght": 400}, "NotoSans-Italic.ttf", True)
    derive(italic, destination, ITALIC_SHA256, {"wdth": 100, "wght": 700}, "NotoSans-BoldItalic.ttf", True)
