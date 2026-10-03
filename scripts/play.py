#!/usr/bin/env python3.11
"""Models the parts of the parks' play equipment that boxes and cylinders do not do
justice to, and exports them as glTF.

Run it with Blender, or with the `bpy` module on the same Python it was built for:

    pip install "numpy<2" bpy
    python3 scripts/play.py

The pitches, courts and playgrounds are built in web/static/map/amenities.js out of
posts, bars and blocks, which is what a goal frame or a swing's legs are. What is not:

  * belt_seat: a swing's seat, a rubber belt sagging between its chains;
  * horse: a spring rider's horse, with a saddle and a handlebar through its head;
  * hoop_net: a basketball net, strings crossing on the way down from the rim;
  * goal_net: a goal's net, a grid of strings over its top, back and sides.

Each is modeled in the amenity's own units (amenities.js), about the point the kit
puts it at, and carries no colors: the kit paints it, as it paints everything else.
The node names are the contract with amenities.js.
"""

# pyright: basic
import math
import os

import bpy

try:  # only importable once bpy has loaded
    import bmesh  # type: ignore[reportMissingImports]
    from mathutils import Matrix, Vector  # type: ignore[reportMissingImports]
except (ImportError, ModuleNotFoundError):
    raise SystemExit("this script must be run with Blender or the bpy module")

HERE: str = os.path.dirname(os.path.abspath(__file__))
OUT: str = os.path.normpath(os.path.join(HERE, "..", "web", "static", "play.glb"))


# Blender's z is up; the export turns it into three.js's y. Each part is written in
# three.js's axes - x along the amenity, y up, z across it - and turned on the way in.
def blender(x: float, y: float, z: float) -> Vector:
    return Vector((x, -z, y))


def strand(bm, a, b, r: float, sides: int = 4):
    """A string or a rod from a to b (three.js axes), `r` thick."""
    a, b = blender(*a), blender(*b)
    axis = b - a
    length = axis.length
    if length < 1e-9:
        return
    out = bmesh.ops.create_cone(
        bm, cap_ends=False, segments=sides, radius1=r, radius2=r, depth=length
    )
    turn = axis.normalized().to_track_quat("Z", "Y").to_matrix().to_4x4()
    bmesh.ops.transform(
        bm, matrix=Matrix.Translation((a + b) / 2) @ turn, verts=out["verts"]
    )


def blob(bm, at, size, segments: int = 10):
    """An ellipsoid at `at`, `size` its half extents (three.js axes)."""
    out = bmesh.ops.create_uvsphere(
        bm, u_segments=segments, v_segments=max(4, segments // 2), radius=1
    )
    sx, sy, sz = size
    scale = Matrix.Diagonal((sx, sz, sy, 1))
    bmesh.ops.transform(
        bm, matrix=Matrix.Translation(blender(*at)) @ scale, verts=out["verts"]
    )


def belt_seat(bm):
    """A swing's seat: a belt 0.06 wide sagging between its chains at z = +-0.04, its
    middle at the origin, the chains' ends at its corners."""
    rows, cols = 9, 3
    grid = []
    for i in range(rows):
        z = -0.05 + 0.1 * i / (rows - 1)
        sag = 0.012 * (1 - (z / 0.05) ** 2)
        row = []
        for j in range(cols):
            x = -0.03 + 0.06 * j / (cols - 1)
            for up in (0.0, -0.006):
                row.append(bm.verts.new(blender(x, -sag + up, z)))
        grid.append(row)
    for i in range(rows - 1):
        for j in range(cols - 1):
            for k in (0, 1):  # the top and the underside
                a, b = grid[i][2 * j + k], grid[i][2 * (j + 1) + k]
                c, d = grid[i + 1][2 * (j + 1) + k], grid[i + 1][2 * j + k]
                bm.faces.new((a, b, c, d) if k == 0 else (d, c, b, a))
    for side in (-1, 1):  # the clamps the chains end in
        strand(bm, (0, 0, side * 0.04), (0, 0.012, side * 0.04), 0.005, 6)


def horse(bm):
    """A spring rider's horse facing +x, standing on its spring's top at y = 0.075."""
    blob(bm, (0, 0.105, 0), (0.065, 0.032, 0.03), 12)  # the body
    strand(bm, (0.045, 0.12, 0), (0.075, 0.16, 0), 0.016, 8)  # the neck
    blob(bm, (0.085, 0.165, 0), (0.03, 0.017, 0.016), 10)  # the head
    for side in (-1, 1):
        strand(
            bm, (0.075, 0.18, side * 0.008), (0.07, 0.195, side * 0.01), 0.004, 4
        )  # ears
        for x in (-0.04, 0.04):
            strand(
                bm, (x, 0.09, side * 0.018), (x * 1.1, 0.075, side * 0.022), 0.008, 6
            )  # leg stubs
    strand(bm, (-0.062, 0.115, 0), (-0.085, 0.085, 0), 0.007, 6)  # the tail
    blob(bm, (-0.005, 0.135, 0), (0.025, 0.006, 0.026), 10)  # the saddle
    strand(bm, (0.075, 0.17, -0.032), (0.075, 0.17, 0.032), 0.0035, 6)  # the handlebar


def hoop_net(bm):
    """A net under a rim 0.028 across centered on the origin: its strings cross on the
    way down to a narrower ring 0.045 below."""
    top, bottom, depth, n = 0.028, 0.019, 0.045, 12
    for i in range(n):
        a0 = 2 * math.pi * i / n
        for turn in (1, -1):
            a1 = a0 + turn * 2 * math.pi / n
            strand(
                bm,
                (top * math.cos(a0), 0, top * math.sin(a0)),
                (bottom * math.cos(a1), -depth, bottom * math.sin(a1)),
                0.0009,
                3,
            )
    for k in (1, 2):
        r, y = top + (bottom - top) * k / 3, -depth * k / 3
        for i in range(n):
            a0, a1 = 2 * math.pi * i / n, 2 * math.pi * (i + 1) / n
            strand(
                bm,
                (r * math.cos(a0), y, r * math.sin(a0)),
                (r * math.cos(a1), y, r * math.sin(a1)),
                0.0009,
                3,
            )


def goal_net(bm):
    """The net behind a goal whose mouth is the posts at z = +-0.13 and the bar at
    y = 0.12 on x = 0: over the top to 0.06 back, down the back 0.1 back, and in at the
    sides; a grid of strings 0.02 apart."""
    mouth, bar, top_back, foot_back = 0.13, 0.12, 0.06, 0.1
    step, r = 0.02, 0.0008
    # Lengthwise across the mouth: the top, the back, as one bent string each.
    z = -mouth
    while z <= mouth + 1e-9:
        strand(bm, (0, bar, z), (top_back, bar * 0.92, z), r)
        strand(bm, (top_back, bar * 0.92, z), (foot_back, 0, z), r)
        z += step
    # Across, over the top and down the back.
    for k in range(1, 4):
        x = top_back * k / 3
        strand(bm, (x, bar - 0.01 * k / 3, -mouth), (x, bar - 0.01 * k / 3, mouth), r)
    for k in range(1, 6):
        t = k / 6
        x, y = top_back + (foot_back - top_back) * t, bar * 0.92 * (1 - t)
        strand(bm, (x, y, -mouth), (x, y, mouth), r)
    # The sides, a grid on each.
    for side in (-mouth, mouth):
        for k in range(1, 6):
            y = bar * k / 6
            back = (
                foot_back - (foot_back - top_back) * (y / (bar * 0.92))
                if y <= bar * 0.92
                else top_back
            )
            strand(bm, (0, y, side), (back, y, side), r)
        for k in range(1, 5):
            x = foot_back * k / 5
            height = (
                bar
                if x <= top_back
                else bar * 0.92 * (foot_back - x) / (foot_back - top_back)
            )
            strand(bm, (x, 0, side), (x, height, side), r)


PARTS = {
    "belt_seat": belt_seat,
    "horse": horse,
    "hoop_net": hoop_net,
    "goal_net": goal_net,
}


# Implements: REQ-CITY-041
def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)  # type: ignore[reportAttributeAccessIssue]
    for stray in list(bpy.data.objects):  # type: ignore[reportAttributeAccessIssue]
        bpy.data.objects.remove(stray, do_unlink=True)  # type: ignore[reportAttributeAccessIssue]
    made = []
    for name, build in PARTS.items():
        mesh = bpy.data.meshes.new(name)
        bm = bmesh.new()
        build(bm)
        bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-6)
        bm.to_mesh(mesh)
        bm.free()
        obj = bpy.data.objects.new(name, mesh)
        bpy.context.scene.collection.objects.link(obj)  # type: ignore[reportAttributeAccessIssue]
        made.append(obj)
    bpy.ops.export_scene.gltf(  # type: ignore[reportAttributeAccessIssue]
        filepath=OUT,
        export_format="GLB",
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
