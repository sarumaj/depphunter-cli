#!/usr/bin/env python3.11
"""Dresses the walker's whole body - a person, legs to head, with the hands - in the
three outfits the map's styles dress them in, and exports it as glTF, in
web/static/body.glb: the one model of the walker there is.

Run it with Blender, or with the `bpy` module on the same Python it was built for -
the versions it is built with are pinned in scripts/requirements.txt:

    python3.11 -m pip install -r scripts/requirements.txt
    python3 scripts/body.py

The body is MakeHuman's base mesh, shaped and stood by scripts/human.py; the hands
are the `generic-hand` scripts/hand.py fetches and re-rigs, put on its forearms. The
hand is in the file twice over - whole, as the arm the first-person view holds tools
with (web/static/walk/hands.js), and at the body's scale as the body's own - so the
hands are the same hands however they are seen.

What is ours is the outfits, painted on the body and stood off it where cloth is
(OUTFITS), with a hard hat or a helmet over the head (HEADWEAR) and what is on the
chest (CHEST); and the rig the walk poses (armature), MakeHuman's own skin weights
folded onto it. One rig, three meshes skinned to it:

  * body_city: a T-shirt, shorts to above the knee, socks and sneakers, the hair
    short;
  * body_circuit: an electrician's coverall with a chest pocket, knee pads and
    reflective bands, insulating gloves, work boots and a hard hat;
  * body_galaxy: a spacesuit, bulky and ringed at the joints, with a control panel on
    the chest, a helmet with a visor and moon boots.

Colors are vertex colors (COLOR_0); the map paints with flat colors and so do these.
The bone and mesh names are the contract with web/static/walk/body.js, and the
hand's (hand_rig, hand) with web/static/walk/hands.js.

Re-running this does not reproduce the committed file byte for byte: Blender's glTF
exporter writes the same mesh with slightly different floats each time. The model is
committed for that reason rather than built, and a regenerated one is equivalent
without being identical.
"""

# pyright: basic
import math
import os
import sys
from itertools import pairwise

import bpy

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from hand import build as build_hand
from human import build as build_human

try:  # only importable once bpy has loaded
    import bmesh  # type: ignore[reportMissingImports]
    from mathutils import Vector  # type: ignore[reportMissingImports]
except (ImportError, ModuleNotFoundError):
    raise SystemExit("this script must be run with Blender or the bpy module")

HERE: str = os.path.dirname(os.path.abspath(__file__))
OUT: str = os.path.normpath(os.path.join(HERE, "..", "web", "static", "body.glb"))

# Map units: the walker's eye is 0.45 over their feet, so a unit is about 3.55 m. The
# model stands on the origin facing Blender's +y, which the glTF export turns into
# three.js's -z: the way a walker looks. Its right is +x.

# The hand model's fingers run along -y, the back of the hand along +z and the thumb
# along +x, the arm back up +y; on the body it hangs palm in, thumb forward, along
# the forearm, scaled to the body's own hand - a hand held up to the lens is drawn
# larger than life on purpose.
HAND_LENGTH = 0.213  # the hand model's, wrist to the middle fingertip (scripts/hand.py)
# Where the body's forearm is cut and the hand model's goes on, of the way from the
# wrist to the elbow; and the gap between the two cuts the bridge spans.
GRAFT = 0.45
GRAFT_GAP = 0.004
# What is worn over the hand model's forearm and hand, along it from the wrist (+ the
# fingers): [from, to, how far it stands off, color, fabric] - the layers
# web/static/walk/hands.js cuts for the first-person view, OUTFITS there, so that the
# same hands look the same however they are seen. The thickest layer over a point is
# its color.
HAND_LAYERS: dict[str, list] = {
    "city": [],
    "circuit": [
        (None, -0.035, 0.0035, "#24365a", "twill"),
        (-0.012, None, 0.0018, "#d9772b", "rubber"),
        (-0.05, -0.012, 0.0048, "#c46a24", "rubber"),
    ],
    "galaxy": [
        (None, -0.02, 0.005, "#eef0f4", "suit"),
        (-0.03, None, 0.0025, "#d6d9e2", "rubber"),
        (-0.026, -0.018, 0.0065, "#6ff4ff", "rubber"),
    ],
}

# The fabrics, as web/static/walk/cloth.js numbers them. Each vertex carries its own, as
# the _CLOTH attribute (carried in the color's alpha until it is written).
CLOTH: dict[str, int] = {
    "skin": 0,
    "knit": 1,
    "twill": 2,
    "rubber": 3,
    "suit": 4,
    "leather": 5,
}


def color(hexa: str, kind: str) -> tuple[float, float, float, float]:
    """An sRGB hex color, as linear floats, which is what Blender stores, and the
    fabric it is in the alpha."""
    rgb = tuple(((int(hexa[i : i + 2], 16) / 255) ** 2.2) for i in (1, 3, 5))
    return (*rgb, CLOTH[kind] / 8)  # type: ignore[return-value]


# Bare skin: the color hands.js gives the hand held before the eye, the same hand.
SKIN = color("#c98d63", "skin")
HAIR = "#2b211b"
WHITES = "#efece6"
IRIS = "#3a2a1e"


class Point:
    """What an outfit is decided by at one point of the body: the walk's bone it goes
    with most, where it is, which way it faces, and the body's heights (heights) to
    measure from."""

    def __init__(self, bone: str, co, normal, at: dict):
        self.bone, self.at, self.normal = bone, at, normal
        self.x, self.y, self.z = co

    def between(self, low: float, high: float) -> bool:
        return low <= self.z <= high


# Each outfit: what a point of the body wears - (color, fabric, how far it stands off
# the skin) - or None for bare skin.
def city(p: Point):
    a = p.at
    collar = a["neck"] - 0.006
    if p.bone == "head" and p.z > collar:
        scalp = p.z > a["brow"] or (
            p.z > a["eye"] - 0.014 and p.y < a["head_y"] - 0.012
        )
        return (HAIR, "knit", 0.0012) if scalp else None
    if p.bone in ("foot_L", "foot_R") or p.z < a["ankle"] + 0.006:
        if p.z < 0.005:
            return ("#e8e8e8", "rubber", 0.004)  # the sole
        if p.y > a["toe_y"] - 0.018:
            return ("#d8433a", "rubber", 0.004)  # the toe cap
        return ("#f4f4f2", "knit", 0.004)
    if p.z < a["ankle"] + 0.022:
        return ("#f2f2f2", "knit", 0.0015)  # socks
    sleeve = p.bone.startswith("upperarm") and p.z > a["shoulder"] - 0.03
    body = p.bone in ("spine", "hips", "head") and p.z > a["hip"] + 0.02
    if sleeve or body:
        # Up to the collar, round the neck.
        return ("#2a9d8f", "knit", 0.0025) if p.z < collar else None
    if p.z > a["knee"] + 0.02 and not p.bone.startswith(
        ("upperarm", "forearm", "hand")
    ):
        hem = p.z < a["knee"] + 0.028
        return ("#2c3a52" if hem else "#34455f", "twill", 0.0045)
    return None


def circuit(p: Point):
    a = p.at
    if p.z > a["neck"] - 0.004 or p.bone.startswith("hand"):
        return None  # the collar's open
    if p.bone in ("foot_L", "foot_R") or p.z < a["ankle"] + 0.03:
        if p.z < 0.006:
            return ("#141210", "rubber", 0.006)
        if p.y > a["toe_y"] - 0.02 and p.z < a["ankle"] + 0.01:
            return ("#8a6d45", "leather", 0.006)  # the toe cap
        return ("#2a2420", "leather", 0.006)
    if p.between(a["chest"] - 0.005, a["chest"] + 0.005) and p.bone == "spine":
        return ("#d8e04a", "rubber", 0.0035)  # a reflective band round the chest
    if p.between(a["waist"] - 0.004, a["waist"] + 0.004) and p.bone in (
        "spine",
        "hips",
    ):
        return ("#1b1d22", "leather", 0.0045)  # the belt
    if p.between(a["knee"] - 0.012, a["knee"] + 0.014) and p.normal[1] > 0.25:
        return ("#202326", "rubber", 0.0065)  # knee pads
    shin = a["ankle"] + (a["knee"] - a["ankle"]) * 0.4
    if p.between(shin, shin + 0.009) and p.bone.startswith("shin"):
        return ("#d8e04a", "rubber", 0.0035)
    return ("#24365a", "twill", 0.003)


def galaxy(p: Point):
    a = p.at
    if p.bone.startswith("hand"):
        return None
    if p.bone == "head" and p.z > a["neck"] + 0.008:
        return None  # inside the helmet
    if p.bone in ("foot_L", "foot_R") or p.z < a["ankle"] + 0.04:
        if p.z < 0.008:
            return ("#5a5f6b", "rubber", 0.011)
        return ("#c8ccd6", "suit", 0.011)  # moon boots
    # Each ring round the part it belongs to: the arms hang beside the waist, and the
    # elbows are level with the belly.
    rings = (
        (a["neck"] - 0.004, a["neck"] + 0.008, 0.009, ""),  # where the helmet locks on
        (a["waist"] - 0.003, a["waist"] + 0.003, 0.008, ("spine", "hips")),
        (a["knee"] - 0.008, a["knee"] + 0.012, 0.008, ("thigh", "shin")),
        (a["elbow"] - 0.002, a["elbow"] + 0.008, 0.008, ("upperarm", "forearm")),
    )
    for low, high, stand, bones in rings:
        if p.between(low, high) and p.bone.startswith(bones):
            return ("#9aa3b5", "rubber", stand)
    light = a["ankle"] + (a["knee"] - a["ankle"]) * 0.55
    if p.between(light, light + 0.003) and p.bone.startswith("shin"):
        return ("#6ff4ff", "rubber", 0.0065)  # a line of light round the shin
    return ("#eef0f4", "suit", 0.0065)


OUTFITS = {"city": city, "circuit": circuit, "galaxy": galaxy}

# Where each outfit's edges run round the body, which the mesh is cut along so they
# are straight: each a height, from the body's heights (heights).
CUTS = {
    "city": lambda a: [
        0.005,
        a["ankle"] + 0.006,
        a["ankle"] + 0.022,
        a["knee"] + 0.02,
        a["knee"] + 0.028,
        a["hip"] + 0.02,
        a["shoulder"] - 0.03,
        a["neck"] - 0.006,
        a["brow"],
    ],
    "circuit": lambda a: [
        0.006,
        a["ankle"] + 0.01,
        a["ankle"] + 0.03,
        a["chest"] - 0.005,
        a["chest"] + 0.005,
        a["waist"] - 0.004,
        a["waist"] + 0.004,
        a["knee"] - 0.012,
        a["knee"] + 0.014,
        a["ankle"] + (a["knee"] - a["ankle"]) * 0.4,
        a["ankle"] + (a["knee"] - a["ankle"]) * 0.4 + 0.009,
        a["neck"] - 0.004,
    ],
    "galaxy": lambda a: [
        0.008,
        a["ankle"] + 0.04,
        a["neck"] - 0.004,
        a["neck"] + 0.008,
        a["waist"] - 0.003,
        a["waist"] + 0.003,
        a["knee"] - 0.008,
        a["knee"] + 0.012,
        a["elbow"] - 0.002,
        a["elbow"] + 0.008,
        a["ankle"] + (a["knee"] - a["ankle"]) * 0.55,
        a["ankle"] + (a["knee"] - a["ankle"]) * 0.55 + 0.003,
    ],
}

# What a vertex of MakeHuman's mesh is: skin or an eye.
PARTS = {"skin": 0, "eye": 1}

# Over the head: a hard hat - its color, fabric, how far it stands off the skull, how
# much further its brim reaches out over the brow and how much lower it sits at the
# back - or a spacesuit's helmet, a ball of
# `radius` with a visor.
HEADWEAR: dict[str, dict] = {
    "circuit": {
        "color": "#f2c230",
        "kind": "rubber",
        "stand": 0.004,
        "brim": 0.009,
        "back": 0.007,
    },
    "galaxy": {"color": "#eef0f4", "kind": "suit", "radius": 0.043},
}

# Boxes on the chest - [across, down from the shoulders, half sizes, color] - stood on
# its front: a pocket; a control panel and its lights.
CHEST: dict[str, list] = {
    "city": [],
    "circuit": [(0.026, 0.03, (0.012, 0.0025, 0.009), "#1e2d4c")],
    "galaxy": [
        (0.0, 0.045, (0.022, 0.006, 0.014), "#5a5f6b"),
        (-0.008, 0.041, (0.004, 0.0015, 0.004), "#6ff4ff"),
        (0.008, 0.041, (0.004, 0.0015, 0.004), "#d8433a"),
    ],
}


def heights(human: dict) -> dict:
    """The joints' heights the outfits are measured from, and where the head's middle
    and the toes' front are."""
    j = human["joints"]
    hip = j["upperleg01.L____head"][2]
    neck = j["neck01____head"][2]
    points = human["points"]
    head = [p for p, b in zip(points, human["bone_of"]) if b == "head"]
    return {
        "hip": hip,
        "knee": j["lowerleg01.L____head"][2],
        "ankle": j["foot.L____head"][2],
        "waist": hip + 0.022,
        "chest": hip + (neck - hip) * 0.62,
        "shoulder": j["upperarm01.L____head"][2],
        "elbow": j["lowerarm01.L____head"][2],
        "wrist": j["wrist.L____head"][2],
        "neck": neck,
        "eye": j["eye.L____head"][2],
        "brow": j["eye.L____head"][2] + 0.013,
        "top": max(p[2] for p in points),
        "head_y": sum(p[1] for p in head) / len(head),
        "toe_y": max(p[1] for p in points if p[2] < 0.02),
    }


def box(bm, middle, half, col, colors) -> list:
    """A box at `middle`, `half` its size each way; returns its vertices."""
    vertices = [
        bm.verts.new(tuple(m + h * s for m, h, s in zip(middle, half, signs)))
        for signs in (
            (-1, -1, -1),
            (1, -1, -1),
            (1, 1, -1),
            (-1, 1, -1),
            (-1, -1, 1),
            (1, -1, 1),
            (1, 1, 1),
            (-1, 1, 1),
        )
    ]
    for v in vertices:
        colors[v] = col
    for face in (
        (0, 3, 2, 1),
        (4, 5, 6, 7),
        (0, 1, 5, 4),
        (1, 2, 6, 5),
        (2, 3, 7, 6),
        (3, 0, 4, 7),
    ):
        bm.faces.new([vertices[i] for i in face])
    return vertices


class Hands:
    """Where the hand model goes on the body: each side's wrist, the turn from a hand
    hanging straight down to one along that forearm, and the scale."""

    def __init__(self, human: dict):
        j = human["joints"]
        self.scale = (
            Vector(j["wrist.L____head"]) - Vector(j["finger3-3.L____tail"])
        ).length / HAND_LENGTH
        self.wrist, self.turn, self.forearm = {}, {}, {}
        for side, s in ((-1.0, "L"), (1.0, "R")):
            elbow = Vector(j[f"lowerarm01.{s}____head"])
            wrist = Vector(j[f"wrist.{s}____head"])
            self.wrist[side] = wrist
            self.forearm[side] = (elbow - wrist).length
            self.turn[side] = (
                Vector((0, 0, -1)).rotation_difference(wrist - elbow).to_matrix()
            )

    def on_body(self, p, side: float, offset: bool = True) -> Vector:
        """A point of the hand model (or with `offset` false, a direction) where it lies
        on the body: turned to hang palm in with the thumb forward, mirrored for the
        left hand, along the forearm on `side` from its wrist, at the body's scale."""
        q = Vector((p.z, p.x, p.y)) * (self.scale if offset else 1.0)
        q.x *= side
        q = self.turn[side] @ q
        return q + self.wrist[side] if offset else q

    def cut(self, side: float) -> float:
        """How far up the forearm from the wrist the hand model is cut, in its units."""
        return GRAFT * self.forearm[side] / self.scale


def hand_into(bm, hand, hands: Hands, side: float, style: str, colors: dict):
    """The hand model's forearm and hand, cut part way up the forearm, added to `bm`
    on the arm on `side`, painted as worn in `style`; returns each new vertex's
    weights by the hand model's bone names, and the cut's rim."""
    names = [g.name for g in hand.vertex_groups]
    made, weights = {}, {}
    cut = hands.cut(side)
    for v in hand.data.vertices:
        co = hand.matrix_world @ v.co
        if co.y > cut:
            continue
        nv = bm.verts.new(hands.on_body(co, side))
        colors[nv] = worn_on_hand(style, -co.y)
        made[v.index] = nv
        weights[nv] = {names[g.group]: g.weight for g in v.groups if g.weight > 1e-3}
    for f in hand.data.polygons:
        if all(i in made for i in f.vertices):
            # Mirrored for the left hand, so its faces wound the other way round.
            order = list(f.vertices) if side > 0 else list(reversed(f.vertices))
            bm.faces.new([made[i] for i in order])
    rim = [e for e in bm.edges if e.is_boundary and all(v in weights for v in e.verts)]
    return weights, rim


def worn_on_hand(style: str, along: float):
    """What covers the hand model at `along` from the wrist in `style`: the thickest
    layer of HAND_LAYERS over it, or bare skin."""
    top = None
    for start, end, stand, hexa, kind in HAND_LAYERS[style]:
        if (
            (start is None or along >= start)
            and (end is None or along <= end)
            and (top is None or stand > top[0])
        ):
            top = (stand, hexa, kind)
    return color(top[1], top[2]) if top else SKIN


def mesh_for(name: str, style: str, hand, human: dict, hands: Hands, bones: list):
    """The outfit's mesh, as an object, its vertices weighted to `bones`: the body,
    dressed, its forearms cut and the hand model's put on; what is worn on the head
    and the chest."""
    at = heights(human)
    dress = OUTFITS[style]
    bm = bmesh.new()
    # What each vertex is weighted to, which part of the body it is (PARTS), and each
    # face corner's color and fabric: the colors are a face's, so an outfit's edges
    # are sharp, and they are cut along (CUTS), so its edges are straight.
    deform = bm.verts.layers.deform.verify()
    part = bm.verts.layers.int.new("part")
    paint = bm.loops.layers.float_color.new("Col")
    fabric = bm.loops.layers.float.new("_CLOTH")
    index = {b: k for k, b in enumerate(bones)}

    def weigh(v, weights: dict):
        for b, w in weights.items():
            v[deform][index[b]] = w

    def lay(f, col):
        for loop in f.loops:
            loop[paint] = (*col[:3], 1.0)
            loop[fabric] = round(col[3] * 8)

    points, bone_weights = human["points"], human["weights"]
    graft = at["wrist"] + GRAFT * (at["elbow"] - at["wrist"]) + GRAFT_GAP
    lower = [b for b in bone_weights if b.startswith(("forearm", "hand"))]
    # The body, but for the forearms below the graft, and the hands.
    verts = {}
    for i, p in enumerate(points):
        if p[2] < graft and sum(bone_weights[b][i] for b in lower) > 0.5:
            continue
        v = verts[i] = bm.verts.new(tuple(p))
        weigh(v, {b: float(w[i]) for b, w in bone_weights.items() if w[i] > 1e-3})
        v[part] = PARTS["eye"] if i in human["eyes"] else PARTS["skin"]
    for f in human["faces"]:
        if all(i in verts for i in f):
            bm.faces.new([verts[i] for i in f])
    for z in CUTS[style](at):
        geom = bm.verts[:] + bm.edges[:] + bm.faces[:]
        bmesh.ops.bisect_plane(bm, geom=geom, plane_co=(0, 0, z), plane_no=(0, 0, 1))
    # Dressed: each face painted, and each vertex stood off the skin as far as the
    # cloth over it is thick.
    bm.normal_update()
    normals = {v: v.normal.copy() for v in bm.verts}
    stand: dict = {}
    for f in bm.faces:
        middle = f.calc_center_median()
        parts = [v[part] for v in f.verts]
        if PARTS["eye"] in parts:
            lay(f, color(IRIS if f.normal.y > 0.75 else WHITES, "skin"))
            continue
        weights: dict = {}
        for v in f.verts:
            for k, w in v[deform].items():
                weights[k] = weights.get(k, 0.0) + w
        bone = bones[max(weights, key=weights.get)] if weights else "spine"  # type: ignore[reportArgumentType, reportCallIssue]
        worn = dress(Point(bone, middle, f.normal, at))
        if worn is None:
            lay(f, SKIN)
            continue
        hexa, kind, thick = worn
        lay(f, color(hexa, kind))
        for v in f.verts:
            stand[v] = max(stand.get(v, 0.0), thick)
    for v, thick in stand.items():
        v.co += normals[v] * thick
    body = set(bm.faces)
    # The hands, on the forearms' cuts.
    colors: dict = {}
    for side in (-1.0, 1.0):
        suffix = "R" if side > 0 else "L"
        hand_weights, rim = hand_into(bm, hand, hands, side, style, colors)
        for v, w in hand_weights.items():
            weigh(
                v,
                {f"{'hand' if g == 'wrist' else g}_{suffix}": x for g, x in w.items()},
            )
        body_rim = [
            e
            for e in bm.edges
            if e.is_boundary
            and all(v not in hand_weights and v.co.x * side > 0.03 for v in e.verts)
            and all(abs(v.co.z - graft) < 0.02 for v in e.verts)
        ]
        # Painted at the rim as the body's forearm is there.
        for e in body_rim:
            for v in e.verts:
                face = next(f for f in v.link_faces if f in body)
                loop = next(lp for lp in face.loops if lp.vert is v)
                colors[v] = (*loop[paint][:3], loop[fabric] / 8)
        bmesh.ops.bridge_loops(bm, edges=rim + body_rim)
    # A hard hat or a helmet, and what is on the chest.
    for v in headwear(bm, style, human, at, colors):
        weigh(v, {"head": 1.0})
    for across, down, half, hexa in CHEST[style]:
        z = at["shoulder"] - down
        front = max(
            (
                v.co.y
                for v in verts.values()
                if v.is_valid
                and abs(v.co.x - across) < 0.006
                and abs(v.co.z - z) < 0.006
            ),
            default=0.03,
        )
        middle = (across, front + half[1] * 0.6, z)
        for v in box(bm, middle, half, color(hexa, "rubber"), colors):
            weigh(v, {"spine": 1.0})
    for f in bm.faces:
        if f not in body:
            for loop in f.loops:
                col = colors.get(loop.vert, SKIN)
                loop[paint] = (*col[:3], 1.0)
                loop[fabric] = round(col[3] * 8)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    mesh = bpy.data.meshes.new(name)  # type: ignore[reportAttributeAccessIssue]
    bm.to_mesh(mesh)
    bm.free()
    mesh.shade_smooth()
    mesh.color_attributes.active_color = mesh.color_attributes["Col"]
    obj = bpy.data.objects.new(name, mesh)  # type: ignore[reportAttributeAccessIssue]
    for b in bones:
        obj.vertex_groups.new(name=b)
    bpy.context.scene.collection.objects.link(obj)  # type: ignore[reportAttributeAccessIssue]
    return obj


def headwear(bm, style: str, human: dict, at: dict, colors: dict) -> list:
    """The hard hat or the helmet `style` wears, added to `bm`; returns its vertices."""
    spec = HEADWEAR.get(style)
    if not spec:
        return []
    middle = Vector((0.0, at["head_y"], (at["eye"] + at["top"]) / 2 - 0.004))
    col = color(spec["color"], spec["kind"])
    if "radius" in spec:
        # A helmet: a ball round the head, on the neck ring, its visor dark and a
        # light along its top.
        out = bmesh.ops.create_uvsphere(
            bm, u_segments=24, v_segments=16, radius=spec["radius"]
        )
        visor, light = color("#1b2a40", "rubber"), color("#6ff4ff", "rubber")
        for v in out["verts"]:
            d = v.co.normalized()
            v.co += middle
            colors[v] = (
                (light if d.z > 0.45 else visor)
                if d.y > 0.55 and -0.35 < d.z < 0.55
                else col
            )
        return out["verts"]
    # A hard hat: a shell over the crown, the skull's own shape stood off it, and a brim
    # round its rim, reaching out furthest over the brow.
    rim, top = at["brow"] - 0.002, at["top"]
    skull = [
        p
        for p, b in zip(human["points"], human["bone_of"])
        if b == "head" and p[2] > rim - 0.004
    ]
    segments = 24
    angles = [2 * math.pi * s / segments for s in range(segments)]

    def reach(angle: float, z: float) -> float:
        """How far out from the head's middle the skull is, that way at that height."""
        out = 0.0
        for p in skull:
            if abs(p[2] - z) < 0.003:
                off = math.atan2(p[1] - middle.y, p[0]) - angle
                if (
                    abs((off + math.pi) % (2 * math.pi) - math.pi)
                    < 1.5 * math.pi / segments
                ):
                    out = max(out, math.hypot(p[0], p[1] - middle.y))
        return out

    def smooth(radii: list) -> list:
        """The radii, each eased toward its neighbors round the ring."""
        n = len(radii)
        return [
            sum(
                radii[(k + d) % n] * w
                for d, w in ((-2, 1), (-1, 2), (0, 3), (1, 2), (2, 1))
            )
            / 9
            for k in range(n)
        ]

    def ring(z: float, radii: list, low: float = 1.0) -> list:
        """A ring at `z`, its rim dropped `low` of the way down at the back."""
        made = [
            bm.verts.new(
                (
                    r * math.cos(a),
                    middle.y + r * math.sin(a),
                    z - low * spec["back"] * max(0.0, -math.sin(a)),
                )
            )
            for a, r in zip(angles, radii)
        ]
        for v in made:
            colors[v] = col
        return made

    stand = spec["stand"]
    skin = smooth([reach(a, rim) for a in angles])
    brim = [
        r + stand + spec["brim"] * max(0.0, math.sin(a)) ** 2 + 0.002
        for a, r in zip(angles, skin)
    ]
    rings = [
        ring(rim - 0.001, skin),
        ring(rim - 0.001, brim),
        ring(rim + 0.002, brim),
    ]
    last = [r + stand for r in skin]
    for share in (0.0, 0.3, 0.55, 0.75, 0.9, 0.97):
        z = rim + (top - rim) * share + 0.002
        radii = [reach(a, z) or 0.0 for a in angles]
        radii = smooth([(r + stand) if r else q * 0.85 for r, q in zip(radii, last)])
        rings.append(ring(z, radii, 1 - share))
        last = radii
    for a, b in pairwise(rings):
        for s in range(segments):
            t = (s + 1) % segments
            bm.faces.new((a[s], a[t], b[t], b[s]))
    made = [v for r in rings for v in r]
    for loop, z in ((rings[-1], top + stand), (rings[0], rim - 0.001)):
        end = bm.verts.new((0.0, middle.y, z))
        colors[end] = col
        made.append(end)
        for s in range(segments):
            bm.faces.new((loop[s], loop[(s + 1) % segments], end))
    z = rim + (top - rim) * 0.3
    badge = (0.0, middle.y + reach(math.pi / 2, z) + stand + 0.001, z + 0.002)
    return made + box(
        bm, badge, (0.007, 0.0015, 0.004), color("#1b1d22", "rubber"), colors
    )


def armature(hand, human: dict, hands: Hands):
    """The rig, on MakeHuman's joints: the hips, the spine up from them, the head on the
    neck, and for each side a thigh, a shin and a foot, an upper arm, a forearm, and
    the hand model's hand with its fingers."""
    j = {n: Vector(p) for n, p in human["joints"].items()}
    hand = hand.parent
    data = bpy.data.armatures.new("body_rig")  # type: ignore[reportAttributeAccessIssue]
    rig = bpy.data.objects.new("body_rig", data)  # type: ignore[reportAttributeAccessIssue]
    bpy.context.scene.collection.objects.link(rig)  # type: ignore[reportAttributeAccessIssue]
    bpy.context.view_layer.objects.active = rig  # type: ignore[reportAttributeAccessIssue]
    bpy.ops.object.mode_set(mode="EDIT")  # type: ignore[reportAttributeAccessIssue]
    bones = data.edit_bones

    def bone(name, head, tail, parent=None, connect=False):
        b = bones.new(name)
        b.head, b.tail = head, tail
        b.parent, b.use_connect = parent, connect
        # Every bone rolled the same way, so one axis bends every joint forward.
        b.roll = 0
        return b

    def middle(p):
        return Vector((0.0, p.y, p.z))

    hip = j["upperleg01.L____head"].z
    hips = bone(
        "hips", Vector((0, j["spine05____head"].y, hip)), middle(j["spine03____head"])
    )
    spine = bone("spine", hips.tail, middle(j["neck01____head"]), hips, True)
    bone("head", middle(j["neck02____head"]), middle(j["head____tail"]), spine)
    for side, s in ((-1.0, "L"), (1.0, "R")):
        thigh = bone(
            f"thigh_{s}",
            j[f"upperleg01.{s}____head"],
            j[f"lowerleg01.{s}____head"],
            hips,
        )
        shin = bone(f"shin_{s}", thigh.tail, j[f"foot.{s}____head"], thigh, True)
        bone(f"foot_{s}", shin.tail, j[f"foot.{s}____tail"], shin, True)
        upper = bone(
            f"upperarm_{s}",
            j[f"upperarm01.{s}____head"],
            j[f"lowerarm01.{s}____head"],
            spine,
        )
        bone(f"forearm_{s}", upper.tail, j[f"wrist.{s}____head"], upper, True)
        # The hand model's own bones from the wrist out, as they lie on the body: the
        # wrist is this side's hand, and each finger joint keeps its WebXR name.
        for b in sorted(hand.data.bones, key=lambda b: len(b.parent_recursive)):
            if b.name == "forearm":
                continue
            name = "hand" if b.name == "wrist" else b.name
            new = bones.new(f"{name}_{s}")
            new.head = hands.on_body(hand.matrix_world @ b.head_local, side)
            new.tail = hands.on_body(hand.matrix_world @ b.tail_local, side)
            parent = b.parent.name
            new.parent = bones[
                (
                    f"forearm_{s}"
                    if parent == "forearm"
                    else ("hand" if parent == "wrist" else parent) + f"_{s}"
                )
            ]
            new.align_roll(
                hands.on_body(
                    hand.matrix_world.to_3x3() @ b.matrix_local.to_3x3().col[2],
                    side,
                    offset=False,
                )
            )
    bpy.ops.object.mode_set(mode="OBJECT")  # type: ignore[reportAttributeAccessIssue]
    return rig


def skin(obj, rig):
    """The mesh, its vertices already weighted (mesh_for), on the rig."""
    obj.parent = rig
    modifier = obj.modifiers.new("rig", "ARMATURE")
    modifier.object = rig


def material():
    """One material for all three: white, so the vertex colors are what shows."""
    m = bpy.data.materials.new("cloth")  # type: ignore[reportAttributeAccessIssue]
    m.use_nodes = True
    tree = m.node_tree
    bsdf = tree.nodes.get("Principled BSDF")
    attribute = tree.nodes.new("ShaderNodeVertexColor")
    attribute.layer_name = "Col"
    tree.links.new(attribute.outputs["Color"], bsdf.inputs["Base Color"])
    bsdf.inputs["Roughness"].default_value = 0.85
    return m


# Implements: REQ-WALK-059
def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)  # type: ignore[reportAttributeAccessIssue]
    for stray in list(bpy.data.objects):  # type: ignore[reportAttributeAccessIssue]
        bpy.data.objects.remove(stray, do_unlink=True)  # type: ignore[reportAttributeAccessIssue]
    hand, _ = build_hand()
    human = build_human()
    hands = Hands(human)
    rig = armature(hand, human, hands)
    cloth = material()
    made = []
    bones = [b.name for b in rig.data.bones]
    for style in OUTFITS:
        obj = mesh_for(f"body_{style}", style, hand, human, hands, bones)
        obj.data.materials.append(cloth)
        skin(obj, rig)
        made.append(obj)
    bpy.ops.export_scene.gltf(  # type: ignore[reportAttributeAccessIssue]
        filepath=OUT,
        export_format="GLB",
        export_skins=True,
        export_animations=False,
        export_apply=False,
        export_yup=True,
        use_selection=False,
        # body.js shades the body by normals it works out itself, and hands.js lights
        # the hand by its own; neither reads a texture.
        export_texcoords=False,
        export_vertex_color="ACTIVE",
        export_attributes=True,
    )
    for obj in made:
        print(f"  {obj.name}: {len(obj.data.vertices)} vertices")
    print(f"wrote {OUT}: {os.path.getsize(OUT)} bytes")


if __name__ == "__main__":
    main()
