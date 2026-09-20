"""Models the walker's hand and forearm in Blender and exports it as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 tools/hand.py

It writes web/static/hand.glb, which the UI loads once and clones for every hand it
draws (web/static/tools.js). The mesh is generated rather than sculpted, so it can be
reviewed as source and regenerated: a skeleton of edges with a radius at every joint,
grown into a solid by the Skin modifier and rounded off by Subdivision Surface. That
gives the swell of a forearm, the web between the fingers and the taper of a fingertip
without anyone modelling them by hand.

An armature with one bone per phalanx rides along with it, so the fingers can be
curled at run time: the UI closes the fist around whatever the walker is holding.
"""

import math
import os
import sys

import bpy
from mathutils import Vector

# Blender's axes here: +y is the way the fingers point, +z is the back of the hand,
# +x is the thumb's side of a right hand. The glTF exporter turns that into the
# y-up convention three.js reads.
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "web", "static", "hand.glb")

# The skeleton, joint by joint: a name, where it is, how thick the flesh is there, and
# which joint it grows out of. Lengths are in map units - a building is one across and
# the walker is about 0.55 tall - so a hand is a little under 0.2 long.
JOINTS = [
    # name          position                     radius   parent
    ("elbow", (0.000, -0.430, 0.030), 0.056, None),
    ("forearm", (0.000, -0.250, 0.014), 0.048, "elbow"),
    ("wrist", (0.000, -0.062, 0.000), 0.029, "forearm"),
    ("palm", (0.000, -0.020, 0.000), 0.033, "wrist"),
]

# The knuckles, across the head of the palm. The middle finger sits highest and
# furthest forward, the little finger lowest and shortest, which is what stops a hand
# from reading as a glove.
KNUCKLES = [
    ("f0", (0.034, 0.050, -0.002), 0.0165),  # index
    ("f1", (0.011, 0.056, 0.000), 0.0170),  # middle
    ("f2", (-0.012, 0.052, -0.001), 0.0160),  # ring
    ("f3", (-0.034, 0.042, -0.004), 0.0140),  # little
]
# Each finger's three bones: how long, and how thick it has become by the end of each.
FINGERS = [
    [(0.040, 0.0112), (0.026, 0.0101), (0.020, 0.0082)],
    [(0.045, 0.0115), (0.029, 0.0104), (0.021, 0.0084)],
    [(0.042, 0.0110), (0.027, 0.0099), (0.020, 0.0080)],
    [(0.033, 0.0098), (0.020, 0.0089), (0.017, 0.0072)],
]
# How far each finger splays and lifts as it leaves the knuckle: a hand at rest is a
# shallow fan, not four parallel rods.
SPLAY = [0.10, 0.03, -0.05, -0.13]
LIFT = [0.06, 0.04, 0.02, 0.0]

# The thumb: out of the side of the palm, rotated towards the fingers.
THUMB = [
    ("t0", (0.034, -0.030, 0.004), 0.0195),
    ("t1", (0.068, 0.002, 0.013), 0.0160),
    ("t2", (0.088, 0.036, 0.020), 0.0128),
    ("t3", (0.097, 0.058, 0.024), 0.0098),
]


def clear():
    """An empty file, however bpy was started."""
    bpy.ops.wm.read_factory_settings(use_empty=True)


def skeleton():
    """The joints and the edges between them, with a radius carried per joint."""
    verts, edges, radii, index = [], [], [], {}

    def add(name, pos, radius, parent):
        index[name] = len(verts)
        verts.append(Vector(pos))
        radii.append(radius)
        if parent is not None:
            edges.append((index[parent], index[name]))

    for name, pos, radius, parent in JOINTS:
        add(name, pos, radius, parent)

    # The knuckles are a bar across the head of the palm, each finger branching off its
    # own. Running every finger out of one palm vertex asks the skin modifier to close
    # six limbs at a point, which it does by leaving a hole in the back of the hand.
    for i, parent in ((1, "palm"), (0, "f1_0"), (2, "f1_0"), (3, "f2_0")):
        name, pos, radius = KNUCKLES[i]
        add(name + "_0", pos, radius, parent)
    for i, (name, pos, radius) in enumerate(KNUCKLES):
        at = Vector(pos)
        # Each bone continues the last one, fanning outwards and lifting a little.
        direction = Vector((math.sin(SPLAY[i]), math.cos(SPLAY[i]), math.sin(LIFT[i]))).normalized()
        for j, (length, radius) in enumerate(FINGERS[i]):
            at = at + direction * length
            add(f"{name}_{j + 1}", at, radius, f"{name}_{j}")

    for i, (name, pos, radius) in enumerate(THUMB):
        add(name, pos, radius, "palm" if i == 0 else THUMB[i - 1][0])

    mesh = bpy.data.meshes.new("hand")
    mesh.from_pydata([tuple(v) for v in verts], edges, [])
    mesh.update()
    obj = bpy.data.objects.new("hand", mesh)
    bpy.context.collection.objects.link(obj)
    return obj, radii, index, verts


def grow(obj, radii):
    """Skin the skeleton and round it off, then bake both into the mesh."""
    bpy.context.view_layer.objects.active = obj
    obj.select_set(True)
    skin = obj.modifiers.new("Skin", "SKIN")
    skin.use_smooth_shade = True
    # The Skin modifier reads a radius pair per vertex: the flesh is a little wider
    # across the hand than it is deep, which is why a palm is a slab and not a tube.
    for i, radius in enumerate(radii):
        obj.data.skin_vertices[0].data[i].radius = (radius, radius * 0.86)
    obj.data.skin_vertices[0].data[0].use_root = True

    subsurf = obj.modifiers.new("Subdivision", "SUBSURF")
    subsurf.levels = subsurf.render_levels = 2
    for modifier in ("Skin", "Subdivision"):
        bpy.ops.object.modifier_apply(modifier=modifier)
    # The skin modifier still leaves doubled vertices and the odd unfilled gap where
    # limbs branch; a hole in the back of a hand is very visible.
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.mesh.remove_doubles(threshold=1e-5)
    bpy.ops.mesh.select_all(action="DESELECT")
    bpy.ops.mesh.select_non_manifold()
    bpy.ops.mesh.fill_holes(sides=8)
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.mesh.normals_make_consistent(inside=False)
    bpy.ops.object.mode_set(mode="OBJECT")
    obj.data.validate(verbose=False)
    bpy.ops.object.shade_smooth()
    return obj


def armature(index, verts):
    """One bone per phalanx, named so the UI can find it, along the same skeleton."""
    data = bpy.data.armatures.new("rig")
    rig = bpy.data.objects.new("rig", data)
    bpy.context.collection.objects.link(rig)
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode="EDIT")

    bones = {}

    def bone(name, head, tail, parent):
        b = data.edit_bones.new(name)
        b.head, b.tail = verts[index[head]], verts[index[tail]]
        if parent:
            b.parent = bones[parent]
            b.use_connect = b.parent.tail == b.head
        bones[name] = b
        return b

    bone("forearm", "elbow", "wrist", None)
    bone("hand", "wrist", "palm", "forearm")
    for name, _, _ in KNUCKLES:
        bone(f"{name}_0", f"{name}_0", f"{name}_1", "hand")
        for j in (1, 2):
            bone(f"{name}_{j}", f"{name}_{j}", f"{name}_{j + 1}", f"{name}_{j - 1}")
    for i in range(3):
        bone(THUMB[i][0], THUMB[i][0], THUMB[i + 1][0], "hand" if i == 0 else THUMB[i - 1][0])

    bpy.ops.object.mode_set(mode="OBJECT")
    return rig


def skin(obj, rig):
    """Weight the mesh to the bones, so curling a finger takes the flesh with it."""
    bpy.ops.object.select_all(action="DESELECT")
    obj.select_set(True)
    rig.select_set(True)
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.parent_set(type="ARMATURE_AUTO")


def material(obj):
    """A plain skin material; the UI re-colors and lights it."""
    mat = bpy.data.materials.new("skin")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = (0.62, 0.39, 0.26, 1.0)
    bsdf.inputs["Roughness"].default_value = 0.72
    obj.data.materials.append(mat)


def main():
    clear()
    obj, radii, index, verts = skeleton()
    grow(obj, radii)
    material(obj)
    rig = armature(index, verts)
    skin(obj, rig)
    # bpy leaves a default object behind whatever the factory settings say; only the
    # hand and its rig may be exported.
    for stray in [o for o in bpy.data.objects if o not in (obj, rig)]:
        bpy.data.objects.remove(stray, do_unlink=True)

    out = os.path.normpath(OUT)
    bpy.ops.export_scene.gltf(
        filepath=out,
        export_format="GLB",
        export_skins=True,
        export_animations=False,
        export_apply=False,
        export_yup=True,
        use_selection=False,
    )
    mesh = obj.data
    print(f"wrote {out}: {len(mesh.vertices)} vertices, {len(mesh.polygons)} faces, "
          f"{len(rig.data.bones)} bones, {os.path.getsize(out)} bytes")


if __name__ == "__main__":
    sys.exit(main())
