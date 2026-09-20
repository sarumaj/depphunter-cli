"""Prepares the plants that stand on the city map, and exports them as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 tools/props.py

Like the hand (tools/hand.py), the models are not made here: they are from flo-bit's
low poly nature pack, which is CC0 (see web/static/vendor/README.md). Blobs of
icosahedron read as a bush from a distance and as nothing at all from a street, and
the map is walked down streets.

What the script does is fit them to the map:

  * fetch each model from the pack, pinned by checksum;
  * split it into the trunk and the crown, because the map colors those separately
    and tints a crown per instance;
  * decimate it to something a few thousand instances can afford;
  * stand it on the origin and scale it to the height a prop is on the map;
  * write web/static/props.glb, which city.js loads once and instances.

The node names are the contract with web/static/props.js.
"""

import hashlib
import os
import sys
import tempfile
import urllib.request

import bpy
import bmesh  # only importable once bpy has loaded, so not in alphabetical order
from mathutils import Vector

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(HERE, "..", "web", "static", "props.glb"))

# The pack, pinned. There is no release to pin to, so each file is pinned by its own
# checksum, which is the thing that actually matters.
PACK = (
    "https://raw.githubusercontent.com/flo-bit/low-poly-asset-packs/"
    "main/nature-pack/glb/{}.glb"
)

# What the map plants, what it is, how tall it stands in map units - a building is one
# across - and how many triangles it may cost. Three species so a park is not a
# pattern, one bush, and a stone the galaxy style uses for its rubble.
MODELS = [
    (
        "tree1",
        "common_tree_1",
        "e17c3742e9251af93f03dee12d7d94c3ab15b78d61661e1a86b704f1f725f9ea",
        0.95,
        950,
    ),
    (
        "tree2",
        "common_tree_2",
        "8451a97982c5e43641dce7cbea4624f68f22f6048c1a13f004d21e1e3c7b9d02",
        0.86,
        950,
    ),
    (
        "tree3",
        "pine_tree_1",
        "d37154637ccd55a5ff0cf53df7645f0503fd0b9bd81900bce8bc92624b2e514a",
        0.92,
        700,
    ),
    (
        "bush",
        "bush_5",
        "794874b05994e50cfedd37298fe8fd9b03c2135b802299c447bc8fb42f5c1b7d",
        0.2,
        120,
    ),
    (
        "rock",
        "stone_2",
        "27b75335e445e611bc18b2837e354cef557f2424e5015b47d93d1bbcf655d6e7",
        0.17,
        60,
    ),
]

# Which half of a model is the trunk. The pack names its materials; anything else is
# foliage, which is the part the map recolors.
WOOD = ("trunk", "bark", "wood", "stem")


def fetch(name, digest):
    """One model from the pack, from a local copy if there is one and GitHub otherwise."""
    path = os.path.join(tempfile.gettempdir(), f"low-poly-nature-{name}.glb")
    if not os.path.exists(path):
        url = PACK.format(name)
        print(f"fetching {url}")
        with urllib.request.urlopen(url) as r, open(path, "wb") as f:
            f.write(r.read())
    with open(path, "rb") as f:
        got = hashlib.sha256(f.read()).hexdigest()
    if got != digest:
        raise SystemExit(f"{path}: sha256 {got}, expected {digest}")
    return path


def imported(path):
    """Everything one model brought in, as one mesh per material: a model may keep
    its trunk and its leaves in one mesh, and the two are colored separately here."""
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=path)
    new = [o for o in bpy.data.objects if o not in before]
    # bpy leaves a default object behind whatever the factory settings say, and the
    # pack's files carry empties for the scene they were authored in.
    meshes = [o for o in new if o.type == "MESH" and o.name != "Icosphere"]
    for stray in [o for o in new if o not in meshes]:
        bpy.data.objects.remove(stray, do_unlink=True)
    bpy.ops.object.select_all(action="DESELECT")
    for o in meshes:
        o.select_set(True)
    bpy.context.view_layer.objects.active = meshes[0]
    bpy.ops.mesh.separate(type="MATERIAL")
    out = [o for o in bpy.data.objects if o.select_get()]
    bpy.ops.object.select_all(action="DESELECT")
    return out


def role(obj):
    """Trunk or crown, by what the model calls the material on it."""
    names = [m.name.lower() for m in obj.data.materials if m] + [obj.name.lower()]
    return "stem" if any(w in n for w in WOOD for n in names) else "head"


def join(objs, name):
    """One object out of several, or None if there were none."""
    if not objs:
        return None
    bpy.ops.object.select_all(action="DESELECT")
    for o in objs:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    if len(objs) > 1:
        bpy.ops.object.join()
    out = bpy.context.view_layer.objects.active
    out.name = out.data.name = name
    # One material, so the part exports as one primitive: the UI shades it itself and
    # the models' own colors are not used.
    out.data.materials.clear()
    out.data.materials.append(skin())
    while out.data.uv_layers:
        out.data.uv_layers.remove(out.data.uv_layers[0])
    bpy.ops.object.select_all(action="DESELECT")
    return out


def skin():
    """The one material every part shares; it carries no information the UI reads."""
    return bpy.data.materials.get("prop") or bpy.data.materials.new("prop")


def bounds(objs):
    lo = Vector((1e9,) * 3)
    hi = Vector((-1e9,) * 3)
    for o in objs:
        for corner in o.bound_box:
            w = o.matrix_world @ Vector(corner)
            lo = Vector(map(min, lo, w))
            hi = Vector(map(max, hi, w))
    return lo, hi


def stand(objs, height):
    """Scale the model to the height a prop is on the map and sit it on the origin,
    centred on its own footprint so that turning an instance turns it on the spot."""
    lo, hi = bounds(objs)
    scale = height / max(1e-6, hi.z - lo.z)
    mid = (lo + hi) / 2
    for o in objs:
        o.data.transform(o.matrix_world)
        o.matrix_world.identity()
        for v in o.data.vertices:
            v.co = Vector(
                (
                    (v.co.x - mid.x) * scale,
                    (v.co.y - mid.y) * scale,
                    (v.co.z - lo.z) * scale,
                )
            )
        o.data.update()


def weld(objs):
    """Sew the model back into a surface.

    glTF splits a vertex per normal and per uv, and these models are flat shaded, so
    every triangle arrives with its own three vertices and shares an edge with
    nothing: a tree is not a mesh but two thousand loose triangles. A decimator given
    that can only throw triangles away - it has nothing to collapse them into - which
    is why a thinned crown came out full of holes and floating shards. Welded, a tree
    is seven surfaces and decimating it works the way decimating is supposed to.
    """
    for o in objs:
        bm = bmesh.new()
        bm.from_mesh(o.data)
        bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
        bm.to_mesh(o.data)
        bm.free()
        o.data.update()


def thin(objs, budget):
    """Decimate to a triangle budget. A prop stands on the map a few thousand times
    over, so what it costs is what it costs times a few thousand."""
    total = sum(len(o.data.polygons) for o in objs)
    if total <= budget:
        return
    for o in objs:
        bpy.context.view_layer.objects.active = o
        decimate = o.modifiers.new("Decimate", "DECIMATE")
        decimate.ratio = budget / total
        bpy.ops.object.modifier_apply(modifier="Decimate")


def prepare(prefix, name, digest, height, budget):
    """One prop: fetched, split, decimated and stood on the origin."""
    meshes = imported(fetch(name, digest))
    parts = []
    for part in ("stem", "head"):
        joined = join([o for o in meshes if role(o) == part], f"{prefix}_{part}")
        if joined:
            parts.append(joined)
    stand(parts, height)
    weld(parts)
    thin(parts, budget)
    for o in parts:
        bpy.context.view_layer.objects.active = o
        bpy.ops.object.shade_flat()
    return parts


def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    for stray in list(bpy.data.objects):
        bpy.data.objects.remove(stray, do_unlink=True)

    made = []
    for spec in MODELS:
        made += prepare(*spec)

    bpy.ops.export_scene.gltf(
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
    for o in made:
        print(f"  {o.name}: {len(o.data.polygons)} faces")
    print(f"wrote {OUT}: {len(made)} parts, {os.path.getsize(OUT)} bytes")


if __name__ == "__main__":
    sys.exit(main())
