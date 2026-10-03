#!/usr/bin/env python3.11
"""Models the walker's whole body - legs, torso, arms and a head with a face - in the
three outfits the map's styles dress them in, and exports them as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 scripts/legs.py

Like the beetle (scripts/bug.py) the body is modeled here rather than taken from a
pack, to dress it per style and rig it as the walk poses it. A first-person walker
sees their chest, arms, legs and feet looking down; the head is the camera's, drawn
only for a view from outside it (web/static/walk/legs.js). One rig, three meshes
skinned to it:

  * legs_city: a T-shirt with short sleeves over bare arms, shorts to above the
    knee, bare shins, socks and sneakers;
  * legs_circuit: an electrician's coverall with a chest pocket, knee pads and
    reflective bands, insulating gloves, and work boots with toe caps;
  * legs_galaxy: a spacesuit, bulky, ringed at the joints and the neck, with a
    control panel on the chest and a ring of light at the cuffs, in moon boots.

Colors are vertex colors (COLOR_0); the map paints with flat colors and so do these.
The bone and mesh names are the contract with web/static/walk/legs.js.
"""

# pyright: basic
import math
import os
from itertools import pairwise

import bpy

try:  # only importable once bpy has loaded
    import bmesh  # type: ignore[reportMissingImports]
    from mathutils import Vector  # type: ignore[reportMissingImports]
except (ImportError, ModuleNotFoundError):
    raise SystemExit("this script must be run with Blender or the bpy module")

HERE: str = os.path.dirname(os.path.abspath(__file__))
# Which vertices of each mesh are an arm's, and which side: mesh_for to skin.
ARMS: dict[str, dict[int, float]] = {}
# ... and which are the head's.
HEADS: dict[str, set[int]] = {}
OUT: str = os.path.normpath(os.path.join(HERE, "..", "web", "static", "legs.glb"))

# Map units: the walker's eye is 0.45 over their feet, so a unit is about 3.55 m. The
# model stands on the origin facing Blender's +y, which the glTF export turns into
# three.js's -z: the way a walker looks.
HIP = 0.245  # the hip joints' height
KNEE = 0.13
ANKLE = 0.024
WAIST = 0.29
SHOULDER = 0.41  # under the eye at 0.45; the neck goes on up behind it
# The arms, hanging at the sides: each turns at the shoulder, bends at the elbow and
# the wrist.
ARM_X = 0.08  # the shoulder joints off the middle
ARM_TOP = 0.395
ELBOW = 0.305
WRIST = 0.228
NECK = 0.428  # where the head sits on the neck; the eye is at 0.45, inside it
HAIR = "#4a3426"
EYES = "#1b1d22"
LIPS = "#b5655a"

# The head - [height, half width, half depth, color or None for skin] - from the chin
# to the crown, as each outfit wears it: hair, a hard hat, a helmet. A face on the
# front of it, but for the helmet's visor.
HEADS_WORN: dict[str, list] = {
    "city": [
        (NECK, 0.015, 0.016, None),
        (0.436, 0.022, 0.026, None),
        (0.450, 0.027, 0.031, None),
        (0.466, 0.028, 0.032, None),
        (0.478, 0.028, 0.032, HAIR),
        (0.490, 0.024, 0.028, HAIR),
        (0.498, 0.015, 0.018, HAIR),
    ],
    "circuit": [
        (NECK, 0.015, 0.016, None),
        (0.436, 0.022, 0.026, None),
        (0.450, 0.027, 0.031, None),
        (0.466, 0.028, 0.032, None),
        (0.472, 0.036, 0.040, "#f2c230"),  # a hard hat, its brim
        (0.476, 0.030, 0.034, "#f2c230"),
        (0.492, 0.026, 0.030, "#f2c230"),
        (0.502, 0.016, 0.019, "#f2c230"),
    ],
    "galaxy": [
        (NECK, 0.031, 0.031, "#9aa3b5"),  # the helmet, locked on the neck ring
        (0.440, 0.040, 0.042, "#eef0f4"),
        (0.462, 0.043, 0.045, "#eef0f4"),
        (0.486, 0.038, 0.040, "#eef0f4"),
        (0.504, 0.024, 0.026, "#eef0f4"),
        (0.510, 0.010, 0.011, "#eef0f4"),
    ],
}
# Boxes on the front of the head - [middle, half sizes, color] - for a face, or a visor.
FACES: dict[str, list] = {
    "city": [
        ((-0.011, 0.030, 0.459), (0.004, 0.002, 0.0025), EYES),
        ((0.011, 0.030, 0.459), (0.004, 0.002, 0.0025), EYES),
        ((0.0, 0.033, 0.451), (0.003, 0.004, 0.005), "#d8a083"),  # the nose
        ((0.0, 0.029, 0.441), (0.008, 0.002, 0.0015), LIPS),
        ((-0.011, 0.031, 0.465), (0.006, 0.0015, 0.0012), HAIR),  # brows
        ((0.011, 0.031, 0.465), (0.006, 0.0015, 0.0012), HAIR),
    ],
    "circuit": [
        ((-0.011, 0.030, 0.459), (0.004, 0.002, 0.0025), EYES),
        ((0.011, 0.030, 0.459), (0.004, 0.002, 0.0025), EYES),
        ((0.0, 0.033, 0.451), (0.003, 0.004, 0.005), "#d8a083"),
        ((0.0, 0.029, 0.441), (0.008, 0.002, 0.0015), LIPS),
        ((0.0, 0.040, 0.481), (0.008, 0.002, 0.004), "#1b1d22"),  # the hat's badge
    ],
    "galaxy": [
        ((0.0, 0.040, 0.465), (0.030, 0.006, 0.017), "#1b2a40"),  # the visor
        ((0.0, 0.046, 0.472), (0.020, 0.002, 0.004), "#6ff4ff"),  # a light along it
    ],
}
APART = 0.03  # each hip joint off the middle
SEGMENTS = 14  # round a leg

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

SKIN = (0.88, 0.67, 0.53, 0.0)


def color(hexa: str, kind: str) -> tuple[float, float, float, float]:
    """An sRGB hex color, as linear floats, which is what Blender stores, and the
    fabric it is in the alpha."""
    rgb = tuple(((int(hexa[i : i + 2], 16) / 255) ** 2.2) for i in (1, 3, 5))
    return (*rgb, CLOTH[kind] / 8)  # type: ignore[return-value]


def fabric(outfit: dict, hexa: str) -> str:
    """The fabric an outfit's color is: its own, where the outfit names one."""
    return outfit.get("fabrics", {}).get(hexa, outfit["cloth"])


# Each outfit: the profile of a leg from the waist down - [height, half width, half
# depth, color] - and its feet; the fabric it is mostly made of (cloth), and the colors
# that are another (fabrics).
OUTFITS: dict[str, dict] = {
    "city": {
        "leg": [
            (0.428, 0.014, 0.014, None),  # the neck
            (0.415, 0.016, 0.016, None),
            (0.411, 0.021, 0.019, "#2a9d8f"),  # a T-shirt, from its collar
            (0.405, 0.045, 0.028, "#2a9d8f"),
            (0.395, 0.062, 0.034, "#2a9d8f"),
            (0.370, 0.060, 0.040, "#2a9d8f"),
            (0.335, 0.054, 0.038, "#2a9d8f"),
            (0.305, 0.050, 0.036, "#2a9d8f"),
            (WAIST, 0.050, 0.036, "#2a9d8f"),
            (0.272, 0.050, 0.036, "#2a9d8f"),
            (0.270, 0.050, 0.036, "#34455f"),  # shorts, loose
            (HIP, 0.030, 0.029, "#34455f"),
            (0.19, 0.026, 0.026, "#34455f"),
            (0.152, 0.024, 0.024, "#2c3a52"),
            (0.150, 0.0155, 0.0160, None),  # bare knee and shin
            (KNEE, 0.0150, 0.0160, None),
            (0.085, 0.0140, 0.0155, None),
            (0.045, 0.0100, 0.0110, None),
            (0.042, 0.0105, 0.0115, "#f2f2f2"),  # socks
            (ANKLE, 0.0100, 0.0110, "#f2f2f2"),
        ],
        "shoe": {
            "upper": "#f4f4f2",
            "sole": "#e8e8e8",
            "toe": "#d8433a",
            "height": 0.03,
            "wide": 1.0,
            "fabrics": {"upper": "knit", "sole": "rubber", "toe": "rubber"},
        },
        # An arm from the shoulder down - [height, half width, half depth, color] -
        # round its own middle `arm_x` off the body's: here a T-shirt's sleeve over a
        # bare arm and hand.
        "arm_x": 0.075,
        "arm": [
            (0.405, 0.020, 0.020, "#2a9d8f"),
            (0.390, 0.021, 0.021, "#2a9d8f"),
            (0.360, 0.019, 0.019, "#2a9d8f"),
            (0.358, 0.0145, 0.0145, None),
            (ELBOW, 0.013, 0.013, None),
            (0.270, 0.012, 0.012, None),
            (0.232, 0.0095, 0.0105, None),
            (0.225, 0.008, 0.016, None),
            (0.195, 0.008, 0.017, None),
            (0.178, 0.007, 0.012, None),
        ],
        "cloth": "twill",
        "fabrics": {"#2a9d8f": "knit", "#f2f2f2": "knit"},
    },
    "circuit": {
        "leg": [
            (0.428, 0.014, 0.014, None),  # the neck
            (0.415, 0.016, 0.016, None),
            (0.413, 0.023, 0.021, "#24365a"),  # the coverall, from its collar
            (0.405, 0.047, 0.030, "#24365a"),
            (0.395, 0.063, 0.035, "#24365a"),
            (0.372, 0.061, 0.040, "#24365a"),
            (0.362, 0.060, 0.040, "#d8e04a"),  # a reflective band round the chest
            (0.352, 0.059, 0.040, "#d8e04a"),
            (0.349, 0.058, 0.040, "#24365a"),
            (0.315, 0.053, 0.037, "#24365a"),
            (WAIST, 0.050, 0.036, "#24365a"),  # belted
            (0.276, 0.050, 0.036, "#1b1d22"),
            (0.268, 0.049, 0.035, "#24365a"),
            (HIP, 0.031, 0.030, "#24365a"),
            (0.19, 0.026, 0.026, "#24365a"),
            (0.150, 0.022, 0.023, "#24365a"),
            (0.146, 0.024, 0.026, "#202326"),  # knee pads
            (0.118, 0.024, 0.026, "#202326"),
            (0.114, 0.021, 0.022, "#24365a"),
            (0.075, 0.019, 0.020, "#24365a"),
            (0.072, 0.019, 0.020, "#d8e04a"),  # a reflective band
            (0.062, 0.019, 0.020, "#d8e04a"),
            (0.059, 0.019, 0.020, "#24365a"),
            (0.05, 0.018, 0.019, "#24365a"),
        ],
        "shoe": {
            "upper": "#2a2420",
            "sole": "#141210",
            "toe": "#8a6d45",
            "height": 0.05,
            "wide": 1.12,
            "fabrics": {"upper": "leather", "sole": "rubber", "toe": "leather"},
        },
        # The coverall's sleeve to the wrist, and an insulating glove over it.
        "arm_x": 0.076,
        "arm": [
            (0.405, 0.021, 0.021, "#24365a"),
            (0.390, 0.022, 0.022, "#24365a"),
            (ELBOW, 0.016, 0.016, "#24365a"),
            (0.245, 0.013, 0.013, "#24365a"),
            (0.243, 0.015, 0.015, "#c46a24"),
            (0.224, 0.014, 0.016, "#c46a24"),
            (0.222, 0.009, 0.017, "#d9772b"),
            (0.195, 0.009, 0.018, "#d9772b"),
            (0.178, 0.008, 0.013, "#d9772b"),
        ],
        "cloth": "twill",
        "fabrics": {
            "#1b1d22": "leather",
            "#202326": "rubber",
            "#d8e04a": "rubber",
            "#c46a24": "rubber",
            "#d9772b": "rubber",
        },
        # Boxes on the chest - [middle, half sizes, color]: a pocket.
        "chest": [((0.028, 0.036, 0.382), (0.012, 0.0025, 0.009), "#1e2d4c")],
    },
    "galaxy": {
        "leg": [
            (0.430, 0.026, 0.026, "#2a2f3a"),  # the inside of the neck ring
            (0.430, 0.030, 0.030, "#9aa3b5"),  # the ring the helmet locks on
            (0.414, 0.030, 0.030, "#9aa3b5"),
            (0.412, 0.050, 0.036, "#eef0f4"),  # the suit, bulky
            (0.398, 0.070, 0.042, "#eef0f4"),
            (0.372, 0.068, 0.047, "#eef0f4"),
            (0.335, 0.060, 0.045, "#eef0f4"),
            (0.305, 0.056, 0.042, "#eef0f4"),
            (WAIST, 0.056, 0.042, "#eef0f4"),
            (0.268, 0.055, 0.041, "#9aa3b5"),
            (0.262, 0.055, 0.041, "#eef0f4"),
            (HIP, 0.036, 0.035, "#eef0f4"),
            (0.19, 0.032, 0.032, "#eef0f4"),
            (0.150, 0.029, 0.029, "#eef0f4"),
            (0.146, 0.031, 0.031, "#9aa3b5"),  # a ring at the knee
            (0.122, 0.031, 0.031, "#9aa3b5"),
            (0.118, 0.028, 0.028, "#eef0f4"),
            (0.09, 0.026, 0.026, "#6ff4ff"),  # a line of light down the shin
            (0.086, 0.026, 0.026, "#eef0f4"),
            (0.06, 0.024, 0.024, "#eef0f4"),
        ],
        "shoe": {
            "upper": "#c8ccd6",
            "sole": "#5a5f6b",
            "toe": "#9aa3b5",
            "height": 0.062,
            "wide": 1.3,
            "fabrics": {"upper": "suit", "sole": "rubber", "toe": "rubber"},
        },
        # The suit's sleeve, ringed at the elbow, a ring of light at the cuff, and the
        # glove.
        "arm_x": 0.088,
        "arm": [
            (0.405, 0.026, 0.026, "#eef0f4"),
            (0.390, 0.027, 0.027, "#eef0f4"),
            (0.312, 0.021, 0.021, "#eef0f4"),
            (0.310, 0.023, 0.023, "#9aa3b5"),
            (0.298, 0.023, 0.023, "#9aa3b5"),
            (0.296, 0.020, 0.020, "#eef0f4"),
            (0.246, 0.017, 0.017, "#eef0f4"),
            (0.244, 0.019, 0.019, "#6ff4ff"),
            (0.234, 0.019, 0.019, "#6ff4ff"),
            (0.232, 0.011, 0.019, "#d6d9e2"),
            (0.195, 0.011, 0.020, "#d6d9e2"),
            (0.176, 0.009, 0.014, "#d6d9e2"),
        ],
        "cloth": "suit",
        "fabrics": {
            "#d6d9e2": "rubber",
            "#9aa3b5": "rubber",
            "#6ff4ff": "rubber",
            "#2a2f3a": "rubber",
            "#5a5f6b": "rubber",
        },
        "chest": [
            ((0.0, 0.046, 0.368), (0.022, 0.006, 0.014), "#5a5f6b"),  # a control panel
            ((-0.008, 0.052, 0.372), (0.004, 0.0015, 0.004), "#6ff4ff"),  # its lights
            ((0.008, 0.052, 0.372), (0.004, 0.0015, 0.004), "#d8433a"),
        ],
    },
}


def ring(bm, z: float, cx: float, rx: float, ry: float, col, colors):
    """A ring of vertices round a leg at height z (or round the waist, cx = 0)."""
    out = []
    for i in range(SEGMENTS):
        a = 2 * math.pi * i / SEGMENTS
        v = bm.verts.new((cx + rx * math.cos(a), ry * math.sin(a), z))
        colors[v] = col
        out.append(v)
    return out


def bridge(bm, a: list, b: list):
    for i in range(len(a)):
        j = (i + 1) % len(a)
        bm.faces.new((a[i], a[j], b[j], b[i]))


def cap(bm, loop: list, z: float, cx: float, col, colors):
    middle = bm.verts.new((cx, 0, z))
    colors[middle] = col
    for i in range(len(loop)):
        bm.faces.new((middle, loop[i], loop[(i + 1) % len(loop)]))


def shoe(bm, side: float, spec: dict, colors):
    """A shoe or a boot: an upper round the foot from heel to toe over a sole."""
    wide, top = spec["wide"], spec["height"]
    upper, sole, toe = (
        color(spec[part], spec["fabrics"][part]) for part in ("upper", "sole", "toe")
    )
    cx = side * APART
    # Lengthwise sections: [y, half width, top height], heel to toe.
    sections = [
        (-0.022, 0.011, top),
        (-0.012, 0.013, top),
        (0.0, 0.013, top * 0.9),
        (0.02, 0.0135, top * 0.6),
        (0.04, 0.0135, 0.022),
        (0.055, 0.012, 0.017),
        (0.064, 0.008, 0.013),
    ]
    loops = []
    for k, (y, half, height) in enumerate(sections):
        half *= wide
        col = toe if k >= len(sections) - 2 else upper
        loop = []
        # Round the section: up one side, over the top, down the other, along the sole.
        for i in range(10):
            a = math.pi * i / 9
            v = bm.verts.new(
                (
                    cx + half * math.cos(a),
                    y,
                    0.006 + (height - 0.006) * math.sin(a) ** 0.6,
                )
            )
            colors[v] = col
            loop.append(v)
        for i in range(1, 4):
            v = bm.verts.new((cx - half + 2 * half * i / 4, y, 0.0))
            colors[v] = sole
            loop.append(v)
        loops.append(loop)
    for a, b in pairwise(loops):
        bridge(bm, a, b)
    for loop, z in ((loops[0], 0), (loops[-1], 1)):
        middle = bm.verts.new(
            (
                cx,
                sum(v.co.y for v in loop) / len(loop),
                sum(v.co.z for v in loop) / len(loop),
            )
        )
        colors[middle] = sole if z == 0 else toe
        for i in range(len(loop)):
            a, b = loop[i], loop[(i + 1) % len(loop)]
            bm.faces.new((middle, b, a) if z == 0 else (middle, a, b))


def box(bm, middle: tuple, half: tuple, col, colors):
    """A box at `middle`, `half` its size each way."""
    vertices = [
        bm.verts.new(
            tuple(m + h * s for m, h, s in zip(middle, half, signs, strict=True))
        )
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


def mesh_for(name: str, outfit: dict):
    """The outfit's mesh, its vertex colors written, as an object: a torso from the
    neck down round both hips, closed on top, a leg out of it on each side, and what
    is on its chest."""
    mesh = bpy.data.meshes.new(name)
    bm = bmesh.new()
    colors: dict = {}
    profile = outfit["leg"]
    waist = [r for r in profile if r[0] > HIP + 1e-6]
    paint = lambda hexa: color(hexa, fabric(outfit, hexa)) if hexa else SKIN
    loops = [ring(bm, z, 0.0, rx, ry, paint(col), colors) for z, rx, ry, col in waist]
    # ... down over the tops of the legs, where it is hidden in them.
    _, rx, ry, col = waist[-1]
    loops.append(ring(bm, HIP - 0.012, 0.0, rx * 0.92, ry * 0.95, paint(col), colors))
    for a, b in pairwise(loops):
        bridge(bm, a, b)
    cap(bm, loops[0], waist[0][0], 0.0, paint(waist[0][3]), colors)
    for side in (-1.0, 1.0):
        legs = [
            ring(bm, z, side * APART, rx, ry, paint(col), colors)
            for z, rx, ry, col in profile
            if z <= HIP + 1e-6
        ]
        for a, b in pairwise(legs):
            bridge(bm, a, b)
        bottom = profile[-1]
        cap(
            bm,
            legs[-1],
            bottom[0],
            side * APART,
            paint(bottom[3]),
            colors,
        )
        shoe(bm, side, outfit["shoe"], colors)
    for middle, half, hexa in outfit.get("chest", []):
        box(bm, middle, half, paint(hexa), colors)
    # The arms, out of the shoulders: closed at the top inside them and at the
    # fingertips. Their vertices are weighted to the arm bones (skin), not by height.
    arms = {}
    for side in (-1.0, 1.0):
        before = len(bm.verts)
        rows, x = outfit["arm"], side * outfit["arm_x"]
        loops = [ring(bm, z, x, rx, ry, paint(col), colors) for z, rx, ry, col in rows]
        for a, b in pairwise(loops):
            bridge(bm, a, b)
        for loop, (z, _, _, col) in ((loops[0], rows[0]), (loops[-1], rows[-1])):
            cap(bm, loop, z, x, paint(col), colors)
        arms.update({v: side for v in list(bm.verts)[before:]})
    # The head, closed at the crown and down inside the neck.
    before = len(bm.verts)
    style = name.removeprefix("legs_")
    rows = HEADS_WORN[style]
    loops = [ring(bm, z, 0.0, rx, ry, paint(col), colors) for z, rx, ry, col in rows]
    for a, b in pairwise(loops):
        bridge(bm, a, b)
    for loop, (z, _, _, col) in ((loops[0], rows[0]), (loops[-1], rows[-1])):
        cap(bm, loop, z, 0.0, paint(col), colors)
    for middle, half, hexa in FACES[style]:
        box(
            bm,
            middle,
            half,
            color(hexa, "skin") if hexa != HAIR else paint(hexa),
            colors,
        )
    head = set(list(bm.verts)[before:])
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    bm.verts.index_update()
    ARMS[name] = {v.index: side for v, side in arms.items()}
    HEADS[name] = {v.index for v in head}
    painted = [colors[v] for v in bm.verts]
    bm.to_mesh(mesh)
    bm.free()
    attribute = mesh.color_attributes.new("Col", "BYTE_COLOR", "POINT")
    for i, c in enumerate(painted):
        attribute.data[i].color = (*c[:3], 1.0)
    fabrics = mesh.attributes.new("_CLOTH", "FLOAT", "POINT")
    for i, c in enumerate(painted):
        fabrics.data[i].value = round(c[3] * 8)
    mesh.color_attributes.active_color = attribute
    obj = bpy.data.objects.new(name, mesh)
    bpy.context.scene.collection.objects.link(obj)  # type: ignore[reportAttributeAccessIssue]
    return obj


def armature():
    """The rig: the hips, the spine up from them, and for each side a thigh, a shin and
    a foot, and an upper arm off the spine, a forearm and a hand; and the head on the
    spine."""
    data = bpy.data.armatures.new("legs_rig")
    rig = bpy.data.objects.new("legs_rig", data)
    bpy.context.scene.collection.objects.link(rig)  # type: ignore[reportAttributeAccessIssue]
    bpy.context.view_layer.objects.active = rig  # type: ignore[reportAttributeAccessIssue]
    bpy.ops.object.mode_set(mode="EDIT")  # type: ignore[reportAttributeAccessIssue]
    bones = data.edit_bones
    hips = bones.new("hips")
    hips.head, hips.tail = Vector((0, 0, HIP)), Vector((0, 0, WAIST))
    spine = bones.new("spine")
    spine.head, spine.tail, spine.parent = (
        Vector((0, 0, WAIST)),
        Vector((0, 0, SHOULDER)),
        hips,
    )
    spine.roll = 0
    for side, suffix in ((-1.0, "R"), (1.0, "L")):
        x = side * APART
        thigh = bones.new(f"thigh_{suffix}")
        thigh.head, thigh.tail, thigh.parent = (
            Vector((x, 0, HIP)),
            Vector((x, 0, KNEE)),
            hips,
        )
        shin = bones.new(f"shin_{suffix}")
        shin.head, shin.tail, shin.parent = (
            Vector((x, 0, KNEE)),
            Vector((x, 0, ANKLE)),
            thigh,
        )
        shin.use_connect = True
        foot = bones.new(f"foot_{suffix}")
        foot.head, foot.tail, foot.parent = (
            Vector((x, 0, ANKLE)),
            Vector((x, 0.055, 0.01)),
            shin,
        )
        foot.use_connect = True
        upper = bones.new(f"upperarm_{suffix}")
        upper.head, upper.tail, upper.parent = (
            Vector((side * ARM_X, 0, ARM_TOP)),
            Vector((side * ARM_X, 0, ELBOW)),
            spine,
        )
        forearm = bones.new(f"forearm_{suffix}")
        forearm.head, forearm.tail, forearm.parent = (
            Vector((side * ARM_X, 0, ELBOW)),
            Vector((side * ARM_X, 0, WRIST)),
            upper,
        )
        forearm.use_connect = True
        hand = bones.new(f"hand_{suffix}")
        hand.head, hand.tail, hand.parent = (
            Vector((side * ARM_X, 0, WRIST)),
            Vector((side * ARM_X, 0, 0.17)),
            forearm,
        )
        hand.use_connect = True
        # Every bone rolled the same way, so one axis bends every joint forward.
        for b in (thigh, shin, foot, upper, forearm, hand):
            b.roll = 0
    head = bones.new("head")
    head.head, head.tail, head.parent = (
        Vector((0, 0, NECK)),
        Vector((0, 0, 0.51)),
        spine,
    )
    head.roll = 0
    bpy.ops.object.mode_set(mode="OBJECT")  # type: ignore[reportAttributeAccessIssue]
    return rig


def blend(z: float, at: float, over: float) -> float:
    """0 well above `at`, 1 well below it, eased across `over` either side."""
    t = max(0.0, min(1.0, (at + over - z) / (2 * over)))
    return t * t * (3 - 2 * t)


def skin(obj, rig):
    """Weights by height: the spine over the waist, the hips over the thighs, the thighs
    over the knees, the shins down to the ankles, the feet below them - each eased
    into the next. An arm's vertices go to its own bones, by height along it."""
    groups = {
        name: obj.vertex_groups.new(name=name)
        for name in (
            "spine",
            "hips",
            "thigh_L",
            "shin_L",
            "foot_L",
            "thigh_R",
            "shin_R",
            "foot_R",
            "upperarm_L",
            "forearm_L",
            "hand_L",
            "upperarm_R",
            "forearm_R",
            "hand_R",
            "head",
        )
    }
    arms, heads = ARMS.get(obj.name, {}), HEADS.get(obj.name, set())
    for v in obj.data.vertices:
        x, y, z = v.co
        if v.index in heads:
            groups["head"].add([v.index], 1.0, "REPLACE")
            continue
        if v.index in arms:
            suffix = "L" if arms[v.index] > 0 else "R"
            below_elbow = blend(z, ELBOW, 0.012)
            below_wrist = blend(z, WRIST, 0.006)
            for name, w in (
                (f"upperarm_{suffix}", 1 - below_elbow),
                (f"forearm_{suffix}", below_elbow * (1 - below_wrist)),
                (f"hand_{suffix}", below_elbow * below_wrist),
            ):
                if w > 1e-3:
                    groups[name].add([v.index], w, "REPLACE")
            continue
        suffix = "L" if x > 0 else "R"
        below_waist = blend(z, WAIST + 0.01, 0.01)
        below_hip = blend(z, HIP + 0.012, 0.012)
        below_knee = blend(z, KNEE, 0.012)
        below_ankle = blend(z, ANKLE + 0.012, 0.01) if y < 0.01 else 1.0
        weights = {
            "spine": 1 - below_waist,
            "hips": below_waist * (1 - below_hip),
            f"thigh_{suffix}": below_hip * (1 - below_knee),
            f"shin_{suffix}": below_hip * below_knee * (1 - below_ankle),
            f"foot_{suffix}": below_hip * below_knee * below_ankle,
        }
        for name, w in weights.items():
            if w > 1e-3:
                groups[name].add([v.index], w, "REPLACE")
    obj.parent = rig
    modifier = obj.modifiers.new("rig", "ARMATURE")
    modifier.object = rig


def material():
    """One material for all three: white, so the vertex colors are what shows."""
    m = bpy.data.materials.new("cloth")
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
    rig = armature()
    cloth = material()
    made = []
    for style, outfit in OUTFITS.items():
        obj = mesh_for(f"legs_{style}", outfit)
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
        # legs.js shades by normals it works out itself, and never reads a texture.
        export_normals=False,
        export_texcoords=False,
        export_vertex_color="ACTIVE",
        export_attributes=True,
    )
    for obj in made:
        print(f"  {obj.name}: {len(obj.data.polygons)} faces")
    print(f"wrote {OUT}: {os.path.getsize(OUT)} bytes")


if __name__ == "__main__":
    main()
