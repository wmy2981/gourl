#!/usr/bin/env python3
"""Generate the Android launcher icon resources from assets/favicon.svg.

The brand icon single source is assets/favicon.svg (root CLAUDE.md). This
script converts its <rect> background and <path> glyph into the two Android
resource files the adaptive icon references, so the APK icon can never drift
from the web icon. Output files are gitignored; CI runs this before gradle.

The glyph's full transform chain — the favicon's own mapping
(translate(12 12) scale(0.016) translate(-512 -512), which projects the
512-coordinate path onto the 24 canvas) plus the adaptive outer transform
(scale 6x, glyph center (11.9,13.2) → 108dp canvas center (54,54)) — is
pre-multiplied into a single affine matrix and emitted as ONE <group>
(scaleX/scaleY + translateX/translateY). A flat group avoids the nested
VectorDrawable transform semantics entirely (an early nested-group build
rendered the glyph invisible on device), and the favicon transform stays
lossless: the 512 pathData is carried over verbatim.

The merged matrix works out to T(5.448, -2.352) · S(0.096): the glyph then
spans x[35.7,72.2] y[22,85.5] on the 108dp canvas — fully inside the
adaptive-icon safe zone (diameter-66 center circle), 36.5dp wide (~55% of
the zone, slightly larger than the favicon's 25% so it reads on a launcher).

Usage: python .github/scripts/sync-android-icon.py   (repo root)
"""

import sys
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parent.parent.parent
SRC = ROOT / "assets" / "favicon.svg"
FG_OUT = ROOT / "frontend" / "android" / "app" / "src" / "main" / "res" / "drawable" / "ic_launcher_foreground.xml"
BG_OUT = ROOT / "frontend" / "android" / "app" / "src" / "main" / "res" / "values" / "ic_launcher_background.xml"

NS = {"svg": "http://www.w3.org/2000/svg"}


def mat_mul(a, b):
    """3x3 homogeneous matrix multiply (a · b)."""
    return [
        [sum(a[r][k] * b[k][c] for k in range(3)) for c in range(3)]
        for r in range(3)
    ]


def parse_transform(transform):
    """Parse an SVG transform string into 3x3 matrices, application order."""
    import re

    ops = re.findall(r"(translate|scale)\(([^)]*)\)", transform)
    matrices = []
    for op, args in ops:
        vals = [float(v) for v in args.replace(",", " ").split()]
        if op == "translate":
            tx = vals[0]
            ty = vals[1] if len(vals) > 1 else 0.0
            matrices.append([[1, 0, tx], [0, 1, ty], [0, 0, 1]])
        else:  # scale
            sx = vals[0]
            sy = vals[1] if len(vals) > 1 else sx
            matrices.append([[sx, 0, 0], [0, sy, 0], [0, 0, 1]])
    return matrices


def merged_group(transform):
    """Merge a transform chain into one VectorDrawable group.

    SVG applies the chain right-to-left, so the leftmost op is the outermost
    matrix: M = m1 · m2 · … · mn. A VectorDrawable group applies its scale
    then translate (T(tx,ty) · S(sx,sy) — no rotation/pivot used), so the
    merged matrix is emitted as exactly those two attributes when it has no
    shear/rotation terms.
    """
    m = [[1, 0, 0], [0, 1, 0], [0, 0, 1]]
    for op_mat in parse_transform(transform):
        m = mat_mul(m, op_mat)
    if abs(m[0][1]) > 1e-9 or abs(m[1][0]) > 1e-9:
        sys.exit("favicon transform contains rotation/shear — not representable as scale+translate")
    sx, sy = m[0][0], m[1][1]
    tx, ty = m[0][2], m[1][2]
    return f'<group\n        android:scaleX="{sx:g}"\n        android:scaleY="{sy:g}"\n        android:translateX="{tx:g}"\n        android:translateY="{ty:g}">', sx, sy, tx, ty


def main():
    tree = ET.parse(SRC)
    root = tree.getroot()
    rect = root.find("svg:rect", NS)
    glyph_container = root.find(".//svg:g", NS)
    glyph = glyph_container.find("svg:path", NS) if glyph_container is not None else None
    if rect is None or glyph is None:
        sys.exit("favicon.svg must contain a <rect> background and one <path> glyph")
    bg_color = rect.get("fill") or sys.exit("favicon <rect> has no fill")
    path_data = glyph.get("d") or sys.exit("favicon <path> has no d")
    glyph_fill = glyph.get("fill") or "#FFFFFF"
    # The transform lives on the <g>, not the <path>. Prefix the favicon's own
    # chain with the adaptive outer transform (scale 6x, glyph center
    # (11.9,13.2) → (54,54)); the whole chain collapses into one group.
    adaptive = "translate(-17.4 -25.2) scale(6)"
    glyph_transform = glyph_container.get("transform") or ""
    group, sx, sy, tx, ty = merged_group(f"{adaptive} {glyph_transform}")
    # Sanity: the glyph bounds in the 512 viewBox (the Material link path
    # spans approximately x[315,695] y[254,915]) must project into the
    # adaptive-icon safe zone — the diameter-66 circle around (54,54).
    x0, x1 = 315 * sx + tx, 695 * sx + tx
    y0, y1 = 254 * sy + ty, 915 * sy + ty
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
    if not (21 <= x0 and x1 <= 87 and 21 <= y0 and y1 <= 87):
        sys.exit(
            f"glyph bounds x[{x0:.1f},{x1:.1f}] y[{y0:.1f},{y1:.1f}] leave the safe zone "
            "(center 54,54, radius 33) — fix the adaptive transform"
        )

    fg = f"""<?xml version="1.0" encoding="utf-8"?>
<!-- GENERATED by .github/scripts/sync-android-icon.py from assets/favicon.svg — do not edit.
     The favicon glyph (512-coordinate pathData, carried over verbatim) is
     projected onto the 108dp canvas by one merged group: scale {sx:g}, translate
     ({tx:g}, {ty:g}) — glyph center (54,54), fully inside the adaptive-icon safe zone. -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="108dp"
    android:height="108dp"
    android:viewportWidth="108"
    android:viewportHeight="108">
    {group}
        <path
            android:fillColor="{glyph_fill}"
            android:pathData="{path_data}" />
    </group>
</vector>
"""
    bg = f"""<?xml version="1.0" encoding="utf-8"?>
<!-- GENERATED by .github/scripts/sync-android-icon.py from assets/favicon.svg — do not edit. -->
<resources>
    <color name="ic_launcher_background">{bg_color.upper()}</color>
</resources>
"""
    FG_OUT.write_text(fg, encoding="utf-8")
    BG_OUT.write_text(bg, encoding="utf-8")
    print(f"wrote {FG_OUT.relative_to(ROOT)}")
    print(f"wrote {BG_OUT.relative_to(ROOT)}")
    print(f"merged transform: scale {sx:g}, translate ({tx:g}, {ty:g}); glyph center ({cx:.1f},{cy:.1f})")


if __name__ == "__main__":
    main()
