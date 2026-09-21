"""Builds the beetle a finding walks the streets as, and exports it as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 tools/bug.py

The hand and the plants are models somebody else drew and this fetches (tools/hand.py,
tools/props.py). There is no beetle in either pack, and a beetle is simple enough to
say out loud, so this one is modelled here: a bmesh of spheres and cones, welded,
smoothed where it should be round and left faceted where it should catch the light.

What it is for: a hundred and forty of these walk the map at once, at arm's length
from a walker and as a dot from the map view, drawn unlit in one instanced mesh per
part and tinted per instance by how serious the finding is. So the shape has to read
as a beetle from a metre away in flat paint, and cost almost nothing. What came
before was a squashed sphere with a smaller sphere in front of it and six boxes
under it, which read as a bean.

Three meshes come out, which is the contract with web/static/bugs.js:

  * shell  the wing cases, which take the severity's color
  * dark   the head, the plate behind it, its jaws and its antennae, which stay
           nearly black whatever the severity is
  * legs   six of them, drawn as their own mesh because they rock on their own

Nothing here is colored: bugs.js shades and tints what it is given, the way city.js
does for the plants. It is one beetle standing on the origin, nose along -Y, which
is +Z once the exporter has turned it the way glTF wants.
"""

# pyright: basic
import math
import os

import bpy

try:  # only importable once bpy has loaded
    import bmesh  # type: ignore[reportMissingImports]
    from bpy.types import Object  # type: ignore[reportMissingImports]
    from mathutils import Euler, Vector  # type: ignore[reportMissingImports]
except (ImportError, ModuleNotFoundError):
    raise SystemExit("this script must be run with Blender or the bpy module")

HERE: str = os.path.dirname(os.path.abspath(__file__))
OUT: str = os.path.normpath(os.path.join(HERE, "..", "web", "static", "bug.glb"))

# The beetle, in map units - a building is one across. It keeps the size the boxes it
# replaces had, because the lap a bug walks and how near a shot has to pass are
# measured in bugs.js against that.
LONG: float = 0.115  # half the body, nose to tail
WIDE: float = 0.062  # half of it across the wing cases
TALL: float = 0.052  # and how high they stand off the street


def sphere(bm: bmesh.types.BMesh, r: float, u: int, v: int) -> list[bmesh.types.BMVert]:
    """A sphere at the origin, and the vertices it added."""
    before = set(bm.verts)
    bmesh.ops.create_uvsphere(bm, u_segments=u, v_segments=v, radius=r)  # type: ignore[reportMissingImports]
    return [x for x in bm.verts if x not in before]


def cone(
    bm: bmesh.types.BMesh, r0: float, r1: float, depth: float, segments: int = 6
) -> list[bmesh.types.BMVert]:
    """A cone or cylinder along +Z at the origin, and the vertices it added."""
    before = set(bm.verts)
    bmesh.ops.create_cone(  # type: ignore[reportMissingImports]
        bm,
        cap_ends=True,
        cap_tris=True,
        segments=segments,
        radius1=r0,
        radius2=r1,
        depth=depth,
    )
    return [x for x in bm.verts if x not in before]


def put(
    bm: bmesh.types.BMesh,
    verts: list[bmesh.types.BMVert],
    scale: tuple[float, float, float] = (1, 1, 1),
    rotate: tuple[float, float, float] = (0, 0, 0),
    at: tuple[float, float, float] = (0, 0, 0),
) -> None:
    """Scales, turns and moves what was just added, in that order."""
    bmesh.ops.scale(bm, vec=Vector(scale), verts=verts)  # type: ignore[reportMissingImports]
    if any(rotate):
        bmesh.ops.rotate(
            bm, cent=(0, 0, 0), matrix=Euler(rotate).to_matrix(), verts=verts
        )
    bmesh.ops.translate(bm, vec=Vector(at), verts=verts)  # type: ignore[reportMissingImports]


def mesh(name: str, bm: bmesh.types.BMesh, smooth: bool) -> Object:
    """Closes a bmesh into an object, welded and shaded."""
    # Welded first: the parts are built as separate primitives that overlap, and a
    # decimator or a normal is only as good as the surface it is given. Loose
    # triangles are what made the plants look shredded before tools/props.py sewed
    # them (see that script), and the same applies to anything built from lumps.
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)  # type: ignore[reportMissingImports]
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)  # type: ignore[reportMissingImports]
    data = bpy.data.meshes.new(name)
    bm.to_mesh(data)
    bm.free()
    obj = bpy.data.objects.new(name, data)  # type: ignore[reportMissingImports]
    bpy.context.scene.collection.objects.link(obj)  # type: ignore[reportMissingImports]
    for face in data.polygons:
        face.use_smooth = smooth
    return obj


def shell() -> Object:
    """
    The wing cases, and nothing else: this is the part that takes the color of the
    finding, so the head and its plate are not in it. One shell over two, with the
    seam between them cut in - a beetle seen from above is that seam and the line of
    the shoulders, and without them this is a pebble.
    """
    bm = bmesh.new()

    # The abdomen. Wider at the shoulders than at the tail, which is the silhouette
    # that reads as a beetle rather than as a bean, so it is tapered along its length
    # by hand afterwards rather than being an ellipsoid.
    body = sphere(bm, 1.0, 14, 8)
    # Pushed toward its ends first. A sphere squashed along its length comes to a
    # leaf's point, and a beetle's wing cases are round behind; this widens the
    # cross-sections near the tail without adding a single vertex.
    for vert in body:
        vert.co.y = math.copysign(abs(vert.co.y) ** 0.62, vert.co.y)

    put(bm, body, scale=(WIDE, LONG * 0.8, TALL))
    for vert in body:
        t = (vert.co.y + LONG * 0.8) / (2 * LONG * 0.8)  # 0 at the tail, 1 at the front
        narrow = 0.66 + 0.34 * math.sin(math.pi * min(1.0, t * 1.1))
        vert.co.x *= narrow
        vert.co.z *= 0.72 + 0.3 * narrow

    # And cut square at the shoulders, where the plate covers the join. Wing cases
    # that curve away to a nose leave the beetle a woodlouse.
    front = LONG * 0.58
    for vert in body:
        vert.co.y = min(vert.co.y, front)

    bmesh.ops.translate(bm, vec=Vector((0, LONG * 0.1, TALL * 0.92)), verts=body)  # type: ignore[reportMissingImports]

    # The seam: the two wing cases meet along the middle, and the groove between them
    # is pressed in rather than laid on, so it survives being seen from any angle.
    for vert in body:
        if vert.co.z > TALL * 0.6:
            across = abs(vert.co.x) / WIDE
            vert.co.z -= TALL * 0.32 * math.exp(-(across * across) * 90.0)

    return mesh("shell", bm, smooth=True)


def dark() -> Object:
    """
    The front end, which stays nearly black whatever color the rest of it is: the
    plate behind the head, the head, its jaws and its antennae. Each is narrower
    than the one behind it, so the beetle steps down from shoulders to nose instead
    of tapering into a cone, which is the difference between a beetle and a woodlouse.
    """
    bm = bmesh.new()
    plate = sphere(bm, 1.0, 12, 6)
    put(
        bm,
        plate,
        scale=(WIDE * 0.78, LONG * 0.22, TALL * 0.66),
        at=(0, LONG * 0.84, TALL * 0.74),
    )
    head = sphere(bm, 1.0, 10, 6)
    put(
        bm,
        head,
        scale=(WIDE * 0.44, LONG * 0.18, TALL * 0.5),
        at=(0, LONG * 1.1, TALL * 0.62),
    )
    for side in (-1, 1):
        jaw = cone(bm, WIDE * 0.16, WIDE * 0.03, LONG * 0.3, segments=5)
        put(
            bm,
            jaw,
            rotate=(math.radians(-96), 0, math.radians(18 * side)),
            at=(side * WIDE * 0.17, LONG * 1.3, TALL * 0.6),
        )
        # The antennae: out, forward and a little up, in two lengths, because a
        # beetle's are jointed and a straight spike reads as a horn.
        for lean, length, lift in ((34, 0.5, 20), (52, 0.42, 4)):
            feeler = cone(bm, WIDE * 0.05, WIDE * 0.03, LONG * length, segments=4)
            put(
                bm,
                feeler,
                rotate=(math.radians(-90 + lift), 0, math.radians(lean * side)),
                at=(side * WIDE * 0.3, LONG * 1.16, TALL * 0.82),
            )
    return mesh("dark", bm, smooth=False)


def legs() -> Object:
    """
    Six legs, in three pairs, each a thigh out and down and a shin down to the
    street. They are their own mesh because they rock as one while the shell holds
    still (bugs.js), which is what reads as scurrying at this size.
    """
    bm = bmesh.new()
    # Where each pair sits along the body, how far out it reaches and how far back it
    # sweeps: front legs forward, back legs back, as a beetle's are.
    pairs = (
        (LONG * 0.66, 0.5, -30.0),
        (LONG * 0.06, 0.58, 2.0),
        (-LONG * 0.58, 0.54, 30.0),
    )
    for along, reach, sweep in pairs:
        for side in (-1, 1):
            hip = Vector((side * WIDE * 0.74, along, TALL * 0.44))
            # Out to the knee and a little down, then down to the street: a beetle
            # carries its body close to the ground on bent legs, and a leg that goes
            # straight out to the side is a spider's.
            knee = hip + Vector(
                (
                    side * WIDE * reach,
                    LONG * math.sin(math.radians(sweep)) * 0.7,
                    -TALL * 0.16,
                )
            )
            foot = Vector((knee.x + side * WIDE * 0.22, knee.y + LONG * 0.05, 0.004))
            for a, b, thick in ((hip, knee, 0.13), (knee, foot, 0.09)):
                bone(bm, a, b, WIDE * thick)

    return mesh("legs", bm, smooth=False)


def bone(bm: bmesh.types.BMesh, a: Vector, b: Vector, r: float) -> None:
    """A tapered segment from a to b, thinner at the far end."""
    span = b - a
    verts = cone(bm, r, r * 0.65, span.length, segments=5)
    turn = Vector((0, 0, 1)).rotation_difference(span.normalized()).to_matrix()
    bmesh.ops.rotate(bm, cent=(0, 0, 0), matrix=turn, verts=verts)  # type: ignore[reportMissingImports]
    bmesh.ops.translate(bm, vec=a + span * 0.5, verts=verts)  # type: ignore[reportMissingImports]


def main() -> None:
    bpy.ops.wm.read_factory_settings(use_empty=True)  # type: ignore[reportMissingImports]
    for stray in list(bpy.data.objects):
        bpy.data.objects.remove(stray, do_unlink=True)  # type: ignore[reportMissingImports]

    made = [shell(), dark(), legs()]
    bpy.ops.export_scene.gltf(  # type: ignore[reportMissingImports]
        filepath=OUT,
        export_format="GLB",
        export_skins=False,
        export_animations=False,
        export_apply=False,
        export_yup=True,
        use_selection=False,
        export_normals=False,
        export_texcoords=False,
    )
    for obj in made:
        print(f"  {obj.name}: {len(obj.data.polygons)} faces")

    print(f"wrote {OUT}: {os.path.getsize(OUT)} bytes")


if __name__ == "__main__":
    main()
