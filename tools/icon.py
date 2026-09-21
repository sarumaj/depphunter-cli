"""Draws the depphunter mark, and writes it where the things that want it look.

    python3 tools/icon.py

Three sizes out of one drawing:

  * icon.png            128x128, what vsce puts on the marketplace listing
  * web/static/favicon.png   48x48, the tab icon for browsers that want a raster
  * web/static/favicon.svg   the same mark as vectors, for those that prefer it

The mark is the map itself, shortened to one thing: three towers on an isometric
block, in the blue the map paints an unknown language. Drawn rather than fetched
because it has to be the same shape at 16 pixels and at 128, and because a mark this
simple is a dozen polygons - anything with a gradient or a bevel in it turns to mud in
a 16-pixel tab.
"""

import os

from PIL import Image, ImageDraw

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.normpath(os.path.join(HERE, ".."))

# The faces, lit the way the map lights its buildings: the top brightest, one side a
# shade down, the other darker still.
TOP = (74, 148, 226)
LEFT = (37, 106, 191)
RIGHT = (28, 92, 171)
GROUND_TOP = (126, 196, 122)
GROUND_LEFT = (86, 156, 88)
GROUND_RIGHT = (68, 134, 72)
BACK = (16, 22, 33)

# Where the towers stand on the block and how tall each is, in cells. Three of the
# four cells are built on and one is left as ground, which is what stops the mark
# being a cube and makes it read as a place.
TOWERS = [((0, 0), 1.5), ((1, 0), 0.8), ((0, 1), 1.15)]
PLATE = 0.18  # how thick the block under them is


def iso(x: float, y: float, z: float, unit: float, cx: float, cy: float) -> tuple[float, float]:
    """A point on the 2:1 isometric grid: x to the right and down, y to the left and down."""
    return (cx + (x - y) * unit, cy + (x + y) * unit * 0.5 - z * unit)


def box(draw: ImageDraw.ImageDraw, at: tuple[float, float], size: tuple[float, float],
        z0: float, h: float, unit: float, cx: float, cy: float,
        faces: tuple[tuple[int, int, int], ...]) -> None:
    """One box on the grid: its top, and the two sides that face the viewer."""
    x, y = at
    w, d = size
    p = lambda dx, dy, dz: iso(x + dx, y + dy, dz, unit, cx, cy)
    top, left, right = faces
    hi = z0 + h
    draw.polygon([p(0, 0, hi), p(w, 0, hi), p(w, d, hi), p(0, d, hi)], fill=top)
    draw.polygon([p(0, d, hi), p(w, d, hi), p(w, d, z0), p(0, d, z0)], fill=right)
    draw.polygon([p(w, 0, hi), p(w, d, hi), p(w, d, z0), p(w, 0, z0)], fill=left)


def build(draw: ImageDraw.ImageDraw, unit: float, cx: float, cy: float) -> None:
    """The mark: a block of ground with three towers standing on it."""
    box(draw, (0, 0), (2, 2), 0.0, PLATE, unit, cx, cy, (GROUND_TOP, GROUND_LEFT, GROUND_RIGHT))
    # Back to front, so a nearer tower covers the one behind it.
    for at, h in sorted(TOWERS, key=lambda t: t[0][0] + t[0][1]):
        box(draw, at, (1, 1), PLATE, h, unit, cx, cy, (TOP, LEFT, RIGHT))


def draw_mark(size: int, radius: float) -> Image.Image:
    """The mark at one size, on its rounded ground."""
    # Four times over and scaled down: the only antialiasing polygons get.
    scale = 4
    n = size * scale
    img = Image.new("RGBA", (n, n), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle([0, 0, n - 1, n - 1], radius=radius * scale, fill=BACK)
    build(d, n * 0.175, n / 2, n * 0.605)
    return img.resize((size, size), Image.LANCZOS)


SVG = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="depphunter">
  <rect width="64" height="64" rx="13" fill="#101621"/>
{body}
</svg>
"""


def svg_polygons() -> str:
    """The same drawing as vectors, for the browsers that would rather have those."""
    out: list[str] = []

    class Pen:
        """Stands in for ImageDraw so build() can draw into SVG unchanged."""

        def polygon(self, points, fill):
            d = " ".join(f"{a:.2f},{b:.2f}" for a, b in points)
            out.append(f'  <polygon points="{d}" fill="#{fill[0]:02x}{fill[1]:02x}{fill[2]:02x}"/>')

    build(Pen(), 64 * 0.175, 32.0, 64 * 0.605)  # type: ignore[arg-type]
    return "\n".join(out)


def main() -> None:
    wrote = []
    for path, size, radius in (
        (os.path.join(ROOT, "icon.png"), 128, 26),
        (os.path.join(ROOT, "web", "static", "favicon.png"), 48, 10),
    ):
        draw_mark(size, radius).save(path, optimize=True)
        wrote.append(path)

    svg = os.path.join(ROOT, "web", "static", "favicon.svg")
    with open(svg, "w", encoding="utf-8") as fh:
        fh.write(SVG.format(body=svg_polygons()))
    wrote.append(svg)

    for path in wrote:
        print(f"wrote {os.path.relpath(path, ROOT)}: {os.path.getsize(path)} bytes")


if __name__ == "__main__":
    main()
