"""Draws the depphunter mark, and writes it where the things that want it look.

    python3 tools/icon.py

Three sizes out of one drawing:

  * icon.png            128x128, what vsce puts on the marketplace listing
  * web/static/favicon.png   48x48, the tab icon for browsers that want a raster
  * web/static/favicon.svg   the same mark as vectors, for those that prefer it
  * extension/media/activitybar.svg   the mark in one colour, for VS Code's activity bar
  * extension/media/tree.svg, backpack.svg   the side panel's other views, drawn the same way

The mark is the map itself, shortened to one thing: three towers on an isometric
block, in the blue the map paints an unknown language. Drawn rather than fetched
because it has to be the same shape at 16 pixels and at 128, and because a mark this
simple is a dozen polygons - anything with a gradient or a bevel in it turns to mud in
a 16-pixel tab.
"""

# pyright: basic
import os
from collections.abc import Callable

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


def iso(
    x: float, y: float, z: float, unit: float, cx: float, cy: float
) -> tuple[float, float]:
    """A point on the 2:1 isometric grid: x to the right and down, y to the left and down."""
    return (cx + (x - y) * unit, cy + (x + y) * unit * 0.5 - z * unit)


def box(
    draw: ImageDraw.ImageDraw,
    at: tuple[float, float],
    size: tuple[float, float],
    z0: float,
    h: float,
    unit: float,
    cx: float,
    cy: float,
    faces: tuple[tuple[int, int, int], ...],
) -> None:
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
    box(
        draw,
        (0, 0),
        (2, 2),
        0.0,
        PLATE,
        unit,
        cx,
        cy,
        (GROUND_TOP, GROUND_LEFT, GROUND_RIGHT),
    )
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
    return img.resize((size, size), Image.LANCZOS)  # type: ignore[reportAttributeAccessIssue]


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
            out.append(
                f'  <polygon points="{d}" fill="#{fill[0]:02x}{fill[1]:02x}{fill[2]:02x}"/>'
            )

    build(Pen(), 64 * 0.175, 32.0, 64 * 0.605)  # type: ignore[arg-type]
    return "\n".join(out)


# The activity bar paints its icons itself, in the theme's foreground colour, and
# uses the SVG only as a mask: every colour turns into the same one, and only how
# opaque a pixel is survives. So the faces are told apart by opacity, the way the
# lit drawing tells them apart by brightness. They are painted in greys into a
# luminance mask rather than as translucent shapes, because translucent shapes
# add up where a tower stands in front of another or on the ground, and a mask
# keeps the painter's rule: what is drawn last covers what is behind it.
ACTIVITYBAR = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="8 16 48 48" width="24" height="24">
  <mask id="m" maskUnits="userSpaceOnUse" x="8" y="16" width="48" height="48">
{body}
  </mask>
  <rect x="8" y="16" width="48" height="48" fill="#000" mask="url(#m)"/>
</svg>
"""

FACE_OPACITY = {
    TOP: 1.0,
    LEFT: 0.7,
    RIGHT: 0.5,
    GROUND_TOP: 0.55,
    GROUND_LEFT: 0.35,
    GROUND_RIGHT: 0.25,
}


def activitybar_polygons() -> str:
    """The vector drawing again, as greys for the mask."""
    out: list[str] = []

    class Pen:
        def polygon(self, points, fill):
            d = " ".join(f"{a:.2f},{b:.2f}" for a, b in points)
            g = round(FACE_OPACITY[fill] * 255)
            out.append(f'    <polygon points="{d}" fill="#{g:02x}{g:02x}{g:02x}"/>')

    build(Pen(), 64 * 0.175, 32.0, 64 * 0.605)  # type: ignore[arg-type]
    return "\n".join(out)


def write_activitybar() -> str:
    path = os.path.join(ROOT, "extension", "media", "activitybar.svg")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(ACTIVITYBAR.format(body=activitybar_polygons()))
    return path


# The side panel's other views want icons of their own, and they are painted the
# same way the activity bar's is: as a mask. So they are drawn with the same boxes
# on the same grid, plus the odd stroke a box can't make - a wire, a handle.
WIRE = 1.0  # opacity of a stroke


def view_icon(draw: Callable[[object], None]) -> str:
    """One view icon: draw() at unit size, fitted into the activity bar's frame."""
    shapes: list[tuple[str, list[tuple[float, float]], float]] = []

    class Pen:
        def polygon(self, points, fill):
            shapes.append(("polygon", list(points), FACE_OPACITY[fill]))

        def line(self, points, fill):
            shapes.append(("line", list(points), fill))

    draw(Pen())
    xs = [x for _, pts, _ in shapes for x, _ in pts]
    ys = [y for _, pts, _ in shapes for _, y in pts]
    # The frame is 8,16 48x48; leave the margin the mark leaves.
    k = 42 / max(max(xs) - min(xs), max(ys) - min(ys))
    ox = 32 - (max(xs) + min(xs)) / 2 * k
    oy = 40 - (max(ys) + min(ys)) / 2 * k

    out: list[str] = []
    for kind, pts, opacity in shapes:
        g = round(opacity * 255)
        d = " ".join(f"{x * k + ox:.2f},{y * k + oy:.2f}" for x, y in pts)
        if kind == "polygon":
            out.append(f'    <polygon points="{d}" fill="#{g:02x}{g:02x}{g:02x}"/>')
        else:
            out.append(
                f'    <polyline points="{d}" fill="none" stroke="#{g:02x}{g:02x}{g:02x}"'
                ' stroke-width="2.6" stroke-linejoin="round" stroke-linecap="round"/>'
            )
    return ACTIVITYBAR.format(body="\n".join(out))


def draw_tree(pen) -> None:
    """Dependencies: one block raised behind, wired down to two in front of it."""
    at = lambda x, y, z: iso(x, y, z, 1, 0, 0)
    # The wires first, so the blocks cover where they end.
    pen.line([at(0.5, 0.5, 1.3), at(0.5, 0.5, 0.45), at(2.3, 0.5, 0.45)], WIRE)
    pen.line([at(0.5, 0.5, 0.45), at(0.5, 2.3, 0.45)], WIRE)
    box(pen, (0, 0), (1, 1), 1.3, 0.9, 1, 0, 0, (TOP, LEFT, RIGHT))
    box(pen, (1.8, 0), (1, 1), 0.0, 0.9, 1, 0, 0, (TOP, LEFT, RIGHT))
    box(pen, (0, 1.8), (1, 1), 0.0, 0.9, 1, 0, 0, (TOP, LEFT, RIGHT))


def draw_backpack(pen) -> None:
    """Backpack: a tall block with a flap over its top, a pocket and a handle."""
    at = lambda x, y, z: iso(x, y, z, 1, 0, 0)
    box(pen, (0, 0), (1.1, 0.8), 0.0, 1.9, 1, 0, 0, (TOP, LEFT, RIGHT))
    box(
        pen,
        (-0.04, -0.04),
        (1.18, 0.92),
        1.35,
        0.6,
        1,
        0,
        0,
        (GROUND_TOP, GROUND_LEFT, GROUND_RIGHT),
    )
    box(
        pen,
        (0.2, 0.8),
        (0.7, 0.22),
        0.2,
        0.75,
        1,
        0,
        0,
        (GROUND_TOP, GROUND_LEFT, GROUND_RIGHT),
    )
    # The handle last: it stands on the flap, and nothing is in front of it.
    pen.line(
        [
            at(0.3, 0.4, 1.95),
            at(0.3, 0.4, 2.35),
            at(0.8, 0.4, 2.35),
            at(0.8, 0.4, 1.95),
        ],
        WIRE,
    )


def write_view_icons() -> list[str]:
    wrote = []
    for name, draw in (("tree", draw_tree), ("backpack", draw_backpack)):
        path = os.path.join(ROOT, "extension", "media", f"{name}.svg")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(view_icon(draw))
        wrote.append(path)
    return wrote


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
    wrote.append(write_activitybar())
    wrote.extend(write_view_icons())

    for path in wrote:
        print(f"wrote {os.path.relpath(path, ROOT)}: {os.path.getsize(path)} bytes")


if __name__ == "__main__":
    main()
