// What the walker's modules share (walk.js, and the parts of Walker kept apart from
// it): the walker's measures, what a line can hold on to, and whether the page is to
// keep still.

import * as THREE from '../vendor/three.module.min.js';

// A story is 0.84 units (buildings.js STORY and FACADE): the walker stands a little
// over half as tall, a door a little taller than them.
export const EYE = 0.45;    // eye height above the feet
// Implements: REQ-WALK-004
export const WALK = 3.2, RUN = 8.5; // on foot, units per second
// A curb, a ramp's slope and a bridge's arch are walked, a terrace wall (0.28 in
// layout.js) is not - that takes the ramp or a jump.
// Implements: REQ-WALK-006
export const STEP = 0.15;   // highest ledge walked up without jumping
export const WATER = -0.45; // the water surface (layout LAND_H below the mainland)
// Swimming in the swim ring: how far below the surface the feet hang, the ring on the
// water round the chest and the head out over it.
export const SWIM_SINK = 0.31;
// How fast swimming goes, of walking or running.
// Implements: REQ-TOOL-025
export const SWIM_PACE = 0.55;
export const REACH = 90;    // aiming distance
/** Walker.height's probes for a point rather than a body: what a ball rests on. */
export const POINT = [0, 0];

/** Whether the page has been asked to keep still, which the ways in respect. */
export const reducedMotion = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

/**
 * Whether a line that struck `box` at `at` holds. The ground always does - it is what
 * a shot off a roof is for - and so does anything that is not a building. On a
 * building the grapple's claw holds within its `grip` of the roof's edge and nowhere
 * lower; a fishing hook (`roof`) holds only where it came down on the roof itself,
 * never on a wall. Either way the same shot always does the same thing.
 *
 * Implements: REQ-TOOL-067, REQ-TOOL-069
 */
export function holds(tool, box, at) {
  const line = tool.reel;
  if (!line || !box || box.kind === 'land' || box.kind === 'terrace') return true;
  if (line.grip !== undefined && box.y + box.h - at.y > line.grip) return false;
  return !line.roof || faceOf(box, at).y === 1;
}

/**
 * The outward normal of the face of `box` that `at` is nearest: a side, or the top.
 * The bottom is never struck from outside a box that stands on something.
 */
export function faceOf(box, at) {
  const sides = [
    [box.w / 2 - Math.abs(at.x - box.x), Math.sign(at.x - box.x) || 1, 0, 0],
    [box.d / 2 - Math.abs(at.z - box.z), 0, 0, Math.sign(at.z - box.z) || 1],
    [Math.abs(box.y + box.h - at.y), 0, 1, 0],
  ];
  const [, x, y, z] = sides.reduce((a, b) => (b[0] < a[0] ? b : a));
  return new THREE.Vector3(x, y, z);
}
