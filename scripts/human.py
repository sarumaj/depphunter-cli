"""Prepares the walker's body, for scripts/body.py, which dresses it and exports it in
web/static/body.glb with the hand (scripts/hand.py).

The body is not modeled here. It is MakeHuman's base mesh - a whole human, modeled
and rigged by people who model humans, with skin weights for every joint - which
the MakeHuman project releases under CC0. It is fetched from MakeHuman's repository
at a pinned tag and checked against pinned checksums, the way scripts/hand.py takes
its hand from npm.

What this does is the part that is ours:

  * shape it as MakeHuman's default young man (TARGETS);
  * stand it as the walk poses a body - arms hanging a little out from the sides,
    legs straight under the hips - from the A-pose it is modeled in, with
    dual-quaternion skinning so the shoulders keep their volume;
  * turn it into Blender's axes, facing +y, and scale it so the eyes are 0.45 over
    the soles, as the walk's eye is;
  * fold its 163 bones into the few the walk moves (BONES).

It needs nothing of Blender: build() is plain numpy, and its result is plain data.
"""

# pyright: basic
import gzip
import hashlib
import json
import os
import tempfile
import urllib.request

import numpy as np

# MakeHuman's data, pinned: the tag and each file's sha256.
TAG: str = "v1.2.0"
URL: str = (
    "https://raw.githubusercontent.com/makehumancommunity/makehuman/"
    f"{TAG}/makehuman/data/"
)
FILES: dict[str, str] = {
    "3dobjs/base.obj": "8e761e6624b8f54536409135d1636da63b32486a90d4897f84e121d144f6fb4c",
    "rigs/default.mhskel": "99f179bce0aa850b45d4191a1d0d234c5851f881c057439470ded3bddf729a24",
    "rigs/default_weights.mhw": "118ba978bb538254c67b849c293b2ce1a65e6f56694b04ab38b88a1f1d507832",
}

# Who the body is: MakeHuman's base mesh is a template, every human in it a mix of
# targets over it. These are its default young man's, each with its share - a third
# of each of the three ethnic shapes MakeHuman blends - from MPFB, MakeHuman's
# Blender add-on, whose targets are the same CC0 data compressed.
TARGETS_URL: str = (
    "https://raw.githubusercontent.com/makehumancommunity/mpfb2/"
    "v2.0.8/src/mpfb/data/targets/"
)
TARGETS: dict[str, tuple[str, float]] = {
    "macrodetails/caucasian-male-young.target.gz": (
        "ffe69537e53148edff2776204ba48cf3bf1f3cbc72966592c8a562ab80b07333",
        1 / 3,
    ),
    "macrodetails/african-male-young.target.gz": (
        "05ed84d6df36e788cdfa8da0be15376850524b960cf2f17f3e592b9cf7d21c85",
        1 / 3,
    ),
    "macrodetails/asian-male-young.target.gz": (
        "0928ed8b9f08f60afb9884cf9e9f33a939ed3a85d7f136de9ccc88a76981d6d7",
        1 / 3,
    ),
}

EYE: float = 0.45  # the eyes over the soles, in map units
# How far out from straight down the arms hang: straight down, they would go through
# the hips.
ARM_OUT: float = 0.17  # radians
# Of the turn that brings an arm down, the share the shoulder takes before the upper
# arm does: a shoulder drops as an arm does.
SHOULDER_SHARE: float = 0.3

# The mesh's own groups kept: the body, and the eyeballs MakeHuman fits into it.
KEPT: tuple[str, ...] = ("body", "helper-l-eye", "helper-r-eye")


def fetch(path: str, url: str = URL, digest: str = "") -> str:
    """A pinned file, from a local copy if there is one and its repository otherwise."""
    digest = digest or FILES[path]
    cache = os.path.join(tempfile.gettempdir(), f"makehuman-{digest[:16]}")
    if not os.path.exists(cache):
        print(f"fetching {url}{path}")
        # Downloaded beside it and moved into place whole, so a download cut short is
        # fetched again next time rather than failing its checksum for good.
        with urllib.request.urlopen(url + path) as r, open(cache + ".part", "wb") as f:
            f.write(r.read())
        os.replace(cache + ".part", cache)
    with open(cache, "rb") as f:
        found = hashlib.sha256(f.read()).hexdigest()
    if found != digest:
        raise SystemExit(f"{cache}: sha256 {found}, expected {digest}")
    return cache


def shaped(points):
    """`points` with TARGETS laid over them: each target moves some of the base mesh's
    points, its lines `index dx dy dz`."""
    out = points.copy()
    for path, (digest, share) in TARGETS.items():
        with gzip.open(fetch(path, TARGETS_URL, digest), "rt") as f:
            for line in f:
                if line.startswith("#") or not line.strip():
                    continue
                i, dx, dy, dz = line.split()
                out[int(i)] += share * np.array([float(dx), float(dy), float(dz)])
    return out


def ours(name: str) -> str:
    """The walk's bone (scripts/body.py) a MakeHuman bone goes with: the spine's lowest
    with the hips, the chest's and the shoulders' with the spine, each limb's with its
    own, the face's with the head."""
    base, _, side = name.partition(".")
    suffix = f"_{side}" if side else ""
    if base in ("root", "spine05", "pelvis"):
        return "hips"
    if base.startswith("spine") or base in (
        "breast",
        "clavicle",
        "shoulder01",
        "neck01",
    ):
        return "spine"
    for prefix, bone in (
        ("upperleg", "thigh"),
        ("lowerleg", "shin"),
        ("foot", "foot"),
        ("toe", "foot"),
        ("upperarm", "upperarm"),
        ("lowerarm", "forearm"),
        ("wrist", "hand"),
        ("finger", "hand"),
        ("metacarpal", "hand"),
    ):
        if base.startswith(prefix):
            return bone + suffix
    return "head"


def read_obj(path: str):
    """The vertices, and each group's faces, of a Wavefront file."""
    verts, groups, group = [], {}, None
    with open(path) as f:
        for line in f:
            if line.startswith("v "):
                verts.append([float(t) for t in line.split()[1:4]])
            elif line.startswith("g "):
                group = line.split()[1]
            elif line.startswith("f "):
                face = [int(t.split("/")[0]) - 1 for t in line.split()[1:]]
                groups.setdefault(group, []).append(face)
    return np.array(verts), groups


def turn_between(a, b):
    """The rotation matrix that turns direction a onto direction b."""
    a, b = a / np.linalg.norm(a), b / np.linalg.norm(b)
    axis, cos = np.cross(a, b), float(np.dot(a, b))
    if np.linalg.norm(axis) < 1e-9:
        return np.eye(3)
    k = np.array(
        [[0, -axis[2], axis[1]], [axis[2], 0, -axis[0]], [-axis[1], axis[0], 0]]
    )
    return np.eye(3) + k + k @ k / (1 + cos)


def about(turn, point):
    """A 4x4 transform turning by `turn` about `point`."""
    m = np.eye(4)
    m[:3, :3] = turn
    m[:3, 3] = point - turn @ point
    return m


def partly(turn, share: float):
    """The same turn, `share` of the way."""
    angle = np.arccos(np.clip((np.trace(turn) - 1) / 2, -1, 1))
    if angle < 1e-9:
        return np.eye(3)
    axis = np.array(
        [turn[2, 1] - turn[1, 2], turn[0, 2] - turn[2, 0], turn[1, 0] - turn[0, 1]]
    )
    axis /= np.linalg.norm(axis)
    k = np.array(
        [[0, -axis[2], axis[1]], [axis[2], 0, -axis[0]], [-axis[1], axis[0], 0]]
    )
    a = angle * share
    return np.eye(3) + np.sin(a) * k + (1 - np.cos(a)) * k @ k


def quaternion(m):
    """The unit quaternion (w, x, y, z) of a rotation matrix."""
    w = np.sqrt(max(0.0, 1 + m[0, 0] + m[1, 1] + m[2, 2])) / 2
    x = np.copysign(
        np.sqrt(max(0.0, 1 + m[0, 0] - m[1, 1] - m[2, 2])) / 2, m[2, 1] - m[1, 2]
    )
    y = np.copysign(
        np.sqrt(max(0.0, 1 - m[0, 0] + m[1, 1] - m[2, 2])) / 2, m[0, 2] - m[2, 0]
    )
    z = np.copysign(
        np.sqrt(max(0.0, 1 - m[0, 0] - m[1, 1] + m[2, 2])) / 2, m[1, 0] - m[0, 1]
    )
    return np.array([w, x, y, z])


def qmul(a, b):
    w1, x1, y1, z1 = a.T
    w2, x2, y2, z2 = b.T
    return np.stack(
        [
            w1 * w2 - x1 * x2 - y1 * y2 - z1 * z2,
            w1 * x2 + x1 * w2 + y1 * z2 - z1 * y2,
            w1 * y2 - x1 * z2 + y1 * w2 + z1 * x2,
            w1 * z2 + x1 * y2 - y1 * x2 + z1 * w2,
        ],
        axis=-1,
    )


def dual_quaternion_skin(points, weights, poses):
    """`points` moved by bones blended as dual quaternions, which keeps a joint's
    volume where blending matrices pinches it: `weights` is (points, bones), `poses`
    each bone's 4x4."""
    real = np.array([quaternion(m[:3, :3]) for m in poses])
    dual = np.array(
        [0.5 * qmul(np.array([0.0, *m[:3, 3]]), q) for m, q in zip(poses, real)]
    )
    # Each bone's quaternion on the same side as the vertex's heaviest one's.
    first = real[np.argmax(weights, axis=1)]
    signs = np.sign(np.einsum("pk,bk->pb", first, real))
    signs[signs == 0] = 1
    w = weights * signs
    r = w @ real
    d = w @ dual
    norm = np.linalg.norm(r, axis=1, keepdims=True)
    r, d = r / norm, d / norm
    rv = r[:, 1:]
    rw = r[:, :1]
    # The point turned by r and then moved by 2 d r*.
    turned = points + 2 * np.cross(rv, np.cross(rv, points) + rw * points)
    conj = r * np.array([1, -1, -1, -1])
    moved = 2 * qmul(d, conj)[:, 1:]
    return turned + moved


def build() -> dict:
    """The body, stood, turned and scaled: its points and faces, the walk's bone
    weights for each point, where each of MakeHuman's joints is, which of its bones
    has most of each point, and which points are an eye's."""
    points, groups = read_obj(fetch("3dobjs/base.obj"))
    points = shaped(points)
    with open(fetch("rigs/default.mhskel")) as f:
        skeleton = json.load(f)
    with open(fetch("rigs/default_weights.mhw")) as f:
        mh_weights = json.load(f)["weights"]

    joints = {n: points[ix].mean(axis=0) for n, ix in skeleton["joints"].items()}
    bones = skeleton["bones"]
    head = {n: joints[b["head"]] for n, b in bones.items()}
    tail = {n: joints[b["tail"]] for n, b in bones.items()}
    order: list[str] = []

    def visit(name: str):
        if name not in order:
            if bones[name]["parent"]:
                visit(bones[name]["parent"])
            order.append(name)

    for name in bones:
        visit(name)

    # Where each segment is to point: the arms down and a little out to their side
    # (+x is MakeHuman's left), the legs straight down.
    down = np.array([0.0, -1.0, 0.0])
    aims = {}
    for side, out in (("L", 1.0), ("R", -1.0)):
        arm = np.array([out * np.sin(ARM_OUT), -np.cos(ARM_OUT), 0.0])
        aims[f"upperarm01.{side}"] = (f"upperarm01.{side}", f"upperarm02.{side}", arm)
        aims[f"lowerarm01.{side}"] = (f"lowerarm01.{side}", f"lowerarm02.{side}", arm)
        aims[f"upperleg01.{side}"] = (f"upperleg01.{side}", f"upperleg02.{side}", down)
        aims[f"lowerleg01.{side}"] = (f"lowerleg01.{side}", f"lowerleg02.{side}", down)
    poses: dict[str, np.ndarray] = {}
    for name in order:
        parent = bones[name]["parent"]
        above = poses[parent] if parent else np.eye(4)
        poses[name] = above
        target = aims.get(name)
        if name.startswith("shoulder01."):
            target = aims[f"upperarm01.{name[-1]}"]
        if target is None:
            continue
        start, end, aim = target
        a = (above @ np.append(head[start], 1))[:3]
        b = (above @ np.append(tail[end], 1))[:3]
        turn = turn_between(b - a, aim)
        if name.startswith("shoulder01."):
            turn = partly(turn, SHOULDER_SHARE)
            a = (above @ np.append(head[name], 1))[:3]
        poses[name] = about(turn, a) @ above

    names = list(bones)
    weights = np.zeros((len(points), len(names)))
    for bone, pairs in mh_weights.items():
        k = names.index(bone)
        for vi, w in pairs:
            weights[vi, k] = w
    total = weights.sum(axis=1, keepdims=True)
    weighted = total[:, 0] > 0
    weights[weighted] /= total[weighted]
    stood = points.copy()
    stood[weighted] = dual_quaternion_skin(
        points[weighted], weights[weighted], [poses[n] for n in names]
    )
    stood_joints = {}
    for n, ix in skeleton["joints"].items():
        stood_joints[n] = stood[ix].mean(axis=0)

    # Blender's axes, facing +y: MakeHuman's model faces +z with its left at +x, and
    # here the left is at -x.
    def blender(p):
        return np.stack([-p[..., 0], p[..., 2], p[..., 1]], axis=-1)

    faces = [f for g in KEPT for f in groups[g]]
    used = sorted({i for f in faces for i in f})
    out = blender(stood)
    sole = out[used, 2].min()
    scale = EYE / (blender(stood_joints["eye.L____head"])[2] - sole)
    out[:, 2] -= sole
    out *= scale

    def place(p):
        q = blender(p)
        q[..., 2] -= sole
        return q * scale

    index = {v: k for k, v in enumerate(used)}
    walk = {}
    for k, name in enumerate(names):
        walk.setdefault(ours(name), []).append(k)
    walk_weights = {bone: weights[used][:, ks].sum(axis=1) for bone, ks in walk.items()}
    return {
        "points": out[used],
        # Mirrored and its y and z swapped, which is a turn: wound as it was.
        "faces": [[index[i] for i in f] for f in faces],
        "weights": walk_weights,
        "joints": {n: place(p) for n, p in stood_joints.items()},
        "bone_of": [names[k] for k in np.argmax(weights[used], axis=1)],
        "eyes": {
            index[i]
            for g in ("helper-l-eye", "helper-r-eye")
            for f in groups[g]
            for i in f
        },
    }
