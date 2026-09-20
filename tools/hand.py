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

Re-running this does not reproduce the committed file byte for byte: Blender's glTF
exporter writes the same mesh with slightly different floats each time. The model is
committed for that reason rather than built, and a regenerated one is equivalent
without being identical.
"""

import hashlib
import os
import sys
import tarfile
import tempfile
import urllib.request

import bpy
import bmesh  # only importable once bpy has loaded, so not in alphabetical order
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
ARM = 5.0
# The arm's profile: how far along it each ring sits, and how much wider that ring is
# than the one before. A forearm leaves the wrist narrow and is widest a third of the
# way down - and then keeps going, straight, for as far again as the viewmodel is from
# the lens. That run is not anatomy: the arm has to leave the frame behind the camera
# rather than stop somewhere inside it, or a walker who zooms out sees it end in mid
# air. What closes it off at the end is behind the near plane and is never drawn.
ARM_PROFILE = [
    (0.020, 1.06),
    (0.054, 1.14),
    (0.101, 1.16),
    (0.155, 1.12),
    (0.210, 1.04),
    (0.264, 1.00),
    (0.45, 1.02),
    (0.70, 1.02),
    (0.94, 1.00),
    (0.98, 0.70),
    (1.0, 0.35),
]


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


def forearm(mesh, elbow, wrist):
    """Grow the arm out of the wrist the model was cut off at.

    The model ends in a flat cap across the wrist. Rather than push a separate tube up
    inside it - which leaves the cap's rim showing as a bracelet, and a seam where two
    surfaces that know nothing about each other cross - the cap is taken off and the
    hole it leaves is extruded back along the arm, ring by ring, widening to a forearm
    and closing again at the elbow. What comes out is one surface: there is no join to
    line up because there is no join.
    """
    axis = (elbow - wrist).normalized()
    bm = bmesh.new()
    bm.from_mesh(mesh.data)
    # glTF splits a vertex per normal and per uv, so the model arrives as a shell full
    # of seams that look like holes to anything that walks its edges. Welding them back
    # together costs nothing - the copies sit on top of one another - and leaves one
    # real hole once the cap comes off, which is the one being extruded.
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)

    # The cap: the faces at the far end of the arm that face along it.
    far = max(v.co.dot(axis) for v in bm.verts)
    cap = [
        f
        for f in bm.faces
        if f.calc_center_median().dot(axis) > far - HAND * 0.12
        and f.normal.dot(axis) > 0.6
    ]
    if not cap:
        raise SystemExit("the model does not end in a cap across the wrist")
    bmesh.ops.delete(bm, geom=cap, context="FACES")

    rim = [e for e in bm.edges if len(e.link_faces) == 1]
    ring = {v for e in rim for v in e.verts}
    centre = sum((v.co for v in ring), Vector()) / len(ring)
    # How wide the wrist is where the model stops, which every ring after it is a
    # multiple of: the arm keeps the wrist's own oval rather than becoming a tube.
    width = sum((v.co - centre).length for v in ring) / len(ring)

    edges = rim
    at = centre
    for t, swell in ARM_PROFILE:
        edges, at, ring = grow(
            bm, edges, centre + axis * (HAND * ARM * t) - at, at, swell
        )
    # ... and close the elbow off with a ring pulled into a point.
    bmesh.ops.contextual_create(bm, geom=edges)

    bm.normal_update()
    bm.to_mesh(mesh.data)
    bm.free()
    mesh.data.update()
    return centre.dot(axis), width, axis


def grow(bm, edges, move, at, swell):
    """One ring further along the arm: extrude the open edge, shift it and scale it
    about the arm's own line, so the profile widens without wandering off it."""
    out = bmesh.ops.extrude_edge_only(bm, edges=edges)["geom"]
    verts = [v for v in out if isinstance(v, bmesh.types.BMVert)]
    bmesh.ops.translate(bm, verts=verts, vec=move)
    to = at + move
    bmesh.ops.scale(
        bm,
        verts=verts,
        vec=Vector((swell, swell, swell)),
        space=Matrix.Translation(-to),
    )
    return (
        [
            e
            for e in out
            if isinstance(e, bmesh.types.BMEdge) and len(e.link_faces) == 1
        ],
        to,
        verts,
    )


def weigh(mesh, cuff, width, axis):
    """Weight the new arm: the forearm bone below the cuff, fading into the wrist bone
    across it, so bending the wrist takes the sleeve with it rather than tearing it."""
    forearm_group = mesh.vertex_groups.get("forearm") or mesh.vertex_groups.new(
        name="forearm"
    )
    wrist_group = mesh.vertex_groups["wrist"]
    for v in mesh.data.vertices:
        along = v.co.dot(axis) - cuff
        if along < -width:
            continue  # the hand itself, weighted by the model
        share = min(max(1.0 - along / (width * 2.2), 0.0), 1.0)
        forearm_group.add([v.index], 1.0 - share, "REPLACE")
        wrist_group.add([v.index], share, "REPLACE")
    bpy.context.view_layer.objects.active = mesh
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
    cuff, width, axis = forearm(mesh, elbow, wrist)
    weigh(mesh, cuff, width, axis)

    bpy.ops.export_scene.gltf(
        filepath=OUT,
        export_format="GLB",
        export_skins=True,
        export_animations=False,
        export_apply=False,
        export_yup=True,
        use_selection=False,
        # The UI re-colors the model and never reads a texture, so the coordinates
        # for one are a third of the file wasted.
        export_texcoords=False,
    )
    print(
        f"wrote {OUT}: {len(mesh.data.vertices)} vertices, {len(mesh.data.polygons)} faces, "
        f"{len(rig.data.bones)} bones, {os.path.getsize(OUT)} bytes"
    )


if __name__ == "__main__":
    sys.exit(main())
