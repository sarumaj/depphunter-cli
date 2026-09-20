"""Prepares the walker's hand and forearm, and exports it as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 tools/hand.py

The hand itself is not modelled here. It is `generic-hand` from the WebXR Input
Profiles project - a real hand, modelled and rigged by people who model hands, and
MIT licensed (see web/static/vendor/README.md). Nothing anyone writes in a script
will beat it, and an earlier version of this file spent a lot of lines proving that.
What the script does is the part that is ours:

  * fetch the model from npm, pinned by version and checksum;
  * turn it to the axes the viewmodel is built in, and scale it to map units;
  * give it the forearm it does not have - it is a VR hand, and ends at the wrist;
  * rebuild its rig, because the WebXR profile stores the 25 joints as a flat list
    of poses rather than a skeleton, and a finger that does not carry its own tip
    along when it curls is no use to us;
  * write web/static/hand.glb, which the UI loads once and clones per hand.

The bone names are the WebXR joint names and are the contract with hands.js.
"""

import hashlib
import os
import sys
import tarfile
import tempfile
import urllib.request

import bpy
from mathutils import Matrix, Vector

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.normpath(os.path.join(HERE, "..", "web", "static", "hand.glb"))

# The model, pinned. The package carries a hand per side; we take the right one and
# the UI mirrors it for the left, which is what a left hand is.
PACKAGE = "@webxr-input-profiles/assets"
VERSION = "1.0.20"
TARBALL = f"https://registry.npmjs.org/{PACKAGE}/-/assets-{VERSION}.tgz"
DIGEST = "30df2a2268220fc0d0e034bed1550aabdd7a2500573c5216f64fc70d59c3d91e"
MEMBER = "package/dist/profiles/generic-hand/right.glb"

# The model is built with the fingers along -z, the back of the hand along +x and
# the thumb along +y. The viewmodel wants Blender's -y for the fingers and +z for
# the back, which is the cycle below; glTF's y-up export then lands the fingers on
# +z and the back of the hand on +y, where tools.js expects them.
TO_BLENDER = Matrix(((0, 1, 0), (0, 0, 1), (1, 0, 0))).to_4x4()

# Map units: a building is one across and the walker about 0.55 tall. A viewmodel
# hand is not to scale with the walker - it is close to the lens - and 0.21 long is
# what fills the corner of the frame without swallowing it.
HAND = 0.213

# The digits, each from the palm outwards. Every joint but the last deforms; the tip
# is there to give the distal phalanx a direction to point in.
DIGITS = [
    ["thumb-metacarpal", "thumb-phalanx-proximal", "thumb-phalanx-distal", "thumb-tip"],
] + [
    [
        f"{d}-finger-metacarpal",
        f"{d}-finger-phalanx-proximal",
        f"{d}-finger-phalanx-intermediate",
        f"{d}-finger-phalanx-distal",
        f"{d}-finger-tip",
    ]
    for d in ("index", "middle", "ring", "pinky")
]

# The forearm, as a fraction of the hand's length: how far back the elbow is, and how
# much thicker than the wrist the arm is at its middle and at the elbow.
ARM = 1.42
# How thick the arm is where it leaves the hand, at its widest and at the elbow, as a
# fraction of the hand's length. A forearm is widest a third of the way down from the
# elbow, not at the elbow itself.
SWELL = (0.172, 0.225, 0.192)


def fetch():
    """The upstream model, from a local copy if there is one and npm otherwise."""
    cache = os.path.join(
        tempfile.gettempdir(), f"webxr-input-profiles-assets-{VERSION}.tgz"
    )
    if not os.path.exists(cache):
        print(f"fetching {TARBALL}")
        with urllib.request.urlopen(TARBALL) as r, open(cache, "wb") as f:
            f.write(r.read())
    with open(cache, "rb") as f:
        digest = hashlib.sha256(f.read()).hexdigest()
    if digest != DIGEST:
        raise SystemExit(f"{cache}: sha256 {digest}, expected {DIGEST}")
    out = os.path.join(tempfile.gettempdir(), "generic-hand-right.glb")
    with tarfile.open(cache) as tar:
        with tar.extractfile(MEMBER) as src, open(out, "wb") as dst:
            dst.write(src.read())
    return out


def load(path):
    """The model's mesh and armature, and nothing else that came in with them."""
    bpy.ops.wm.read_factory_settings(use_empty=True)
    for stray in list(bpy.data.objects):
        bpy.data.objects.remove(stray, do_unlink=True)
    bpy.ops.import_scene.gltf(filepath=path)
    rig = next(o for o in bpy.data.objects if o.type == "ARMATURE")
    mesh = next(o for o in rig.children if o.type == "MESH")
    # The profile carries controller widgets alongside the hand, and bpy leaves a
    # default object behind whatever the factory settings say.
    for stray in [o for o in bpy.data.objects if o not in (mesh, rig)]:
        bpy.data.objects.remove(stray, do_unlink=True)
    mesh.name = mesh.data.name = "hand"
    rig.name = rig.data.name = "rig"
    return mesh, rig


def place(mesh, rig):
    """Turn the model to our axes, scale it to map units and sit the wrist on the origin.

    Both the vertices and the bones are moved by hand rather than by transforming the
    objects: the mesh is skinned to the armature, and moving the two together keeps
    the rest pose exactly where it was.
    """
    head = {b.name: b.head_local.copy() for b in rig.data.bones}
    span = (TO_BLENDER @ (head["middle-finger-tip"] - head["wrist"])).length
    scale = Matrix.Scale(HAND / span, 4)
    wrist = scale @ TO_BLENDER @ head["wrist"]
    transform = Matrix.Translation(-wrist) @ scale @ TO_BLENDER

    for v in mesh.data.vertices:
        v.co = transform @ v.co
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode="EDIT")
    for bone in rig.data.edit_bones:
        bone.head, bone.tail = transform @ bone.head, transform @ bone.tail
    bpy.ops.object.mode_set(mode="OBJECT")
    return {name: transform @ p for name, p in head.items()}


def rig_hand(rig, head):
    """Hang the joints off one another, so a finger carries its own tip when it curls.

    WebXR reports every joint as an absolute pose, so the profile's skeleton is a flat
    list of 25 bones with no parents and no direction. Here each joint becomes a bone
    reaching to the next one along its digit, parented to the one before it, and the
    roll is set so that every bone's local x runs across the hand: that one axis is
    what a finger curls about, and hands.js curls them all the same way.
    """
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode="EDIT")
    bones = rig.data.edit_bones

    for digit in DIGITS:
        for i, name in enumerate(digit):
            bone = bones[name]
            bone.parent = bones["wrist"] if i == 0 else bones[digit[i - 1]]
            bone.use_connect = i > 0
            if i + 1 < len(digit):
                bone.tail = head[digit[i + 1]]
            else:  # the tip, carried on past the fingernail so the bone has a length
                bone.tail = bone.head + (bone.head - head[digit[i - 1]]) * 0.35

    # The wrist reaches to the head of the palm rather than off into the arm.
    bones["wrist"].tail = head["middle-finger-metacarpal"]
    # ... and the arm reaches to it, which is the one bone the model does not have:
    # straight on out of the back of the hand, as far again as the hand is long.
    elbow = (
        head["wrist"]
        + (head["wrist"] - head["middle-finger-tip"]).normalized() * HAND * ARM
    )
    arm = bones.new("forearm")
    arm.head, arm.tail = elbow, head["wrist"]
    bones["wrist"].parent, bones["wrist"].use_connect = arm, True

    bpy.ops.armature.select_all(action="SELECT")
    bpy.ops.armature.calculate_roll(type="GLOBAL_POS_Z")
    bpy.ops.object.mode_set(mode="OBJECT")
    return elbow, head["wrist"]


def stump(mesh, axis):
    """The middle of the cut face the model ends at, which is where the arm comes out."""
    far = max((v.co.dot(axis) for v in mesh.data.vertices))
    rim = [v.co for v in mesh.data.vertices if v.co.dot(axis) > far - HAND * 0.03]
    return sum(rim, Vector()) / len(rim)


def forearm(mesh, elbow, wrist):
    """An arm out of the wrist, grown the way the old hand was: a line with a radius
    at every point, thickened by the Skin modifier and rounded off by Subdivision.

    It starts inside the palm and comes out through the cut face the model ends at, so
    there is no seam to line up - the wrist is simply inside the sleeve of the arm.
    """
    axis = (elbow - wrist).normalized()
    centre = stump(mesh, axis) - axis * HAND * 0.28
    points = [centre, centre.lerp(elbow, 0.62), elbow]
    radii = [s * HAND for s in SWELL]

    data = bpy.data.meshes.new("forearm")
    data.from_pydata([tuple(p) for p in points], [(0, 1), (1, 2)], [])
    data.update()
    obj = bpy.data.objects.new("forearm", data)
    bpy.context.collection.objects.link(obj)
    bpy.context.view_layer.objects.active = obj
    obj.select_set(True)
    skin = obj.modifiers.new("Skin", "SKIN")
    skin.use_smooth_shade = True
    for i, r in enumerate(radii):
        # An arm is wider across than it is deep, which is why a forearm reads as a
        # forearm and not as a length of pipe.
        obj.data.skin_vertices[0].data[i].radius = (r, r * 0.84)
    obj.data.skin_vertices[0].data[0].use_root = True

    subsurf = obj.modifiers.new("Subdivision", "SUBSURF")
    subsurf.levels = subsurf.render_levels = 2
    for modifier in ("Skin", "Subdivision"):
        bpy.ops.object.modifier_apply(modifier=modifier)
    obj.data.materials.append(mesh.data.materials[0])
    obj.vertex_groups.new(name="forearm")
    return obj, centre.dot(axis), axis


def attach(mesh, arm, cuff, axis):
    """Join the arm to the hand and weight it, fading into the wrist at the cuff so
    that bending the wrist takes the sleeve with it rather than tearing it."""
    first = len(mesh.data.vertices)
    bpy.ops.object.select_all(action="DESELECT")
    arm.select_set(True)
    mesh.select_set(True)
    bpy.context.view_layer.objects.active = mesh
    bpy.ops.object.join()

    forearm_group = mesh.vertex_groups["forearm"]
    wrist_group = mesh.vertex_groups["wrist"]
    for v in mesh.data.vertices[first:]:
        along = v.co.dot(axis)
        share = min(max((cuff - along) / (HAND * 0.22), 0.0), 1.0)
        forearm_group.add([v.index], 1.0 - share, "REPLACE")
        wrist_group.add([v.index], share, "REPLACE")
    bpy.ops.object.shade_smooth()


def material(mesh):
    """A plain skin material; the UI re-colors and lights it."""
    mat = mesh.data.materials[0]
    mat.name = "skin"
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = (0.62, 0.39, 0.26, 1.0)
    bsdf.inputs["Roughness"].default_value = 0.72


def main():
    mesh, rig = load(fetch())
    head = place(mesh, rig)
    elbow, wrist = rig_hand(rig, head)
    material(mesh)
    arm, cuff, axis = forearm(mesh, elbow, wrist)
    attach(mesh, arm, cuff, axis)

    bpy.ops.export_scene.gltf(
        filepath=OUT,
        export_format="GLB",
        export_skins=True,
        export_animations=False,
        export_apply=False,
        export_yup=True,
        use_selection=False,
    )
    print(
        f"wrote {OUT}: {len(mesh.data.vertices)} vertices, {len(mesh.data.polygons)} faces, "
        f"{len(rig.data.bones)} bones, {os.path.getsize(OUT)} bytes"
    )


if __name__ == "__main__":
    sys.exit(main())
