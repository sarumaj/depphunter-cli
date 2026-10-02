// How a thrown thing flies, and the wind it flies through.
//
// A shot falls under its tool's gravity (tools.js flight: a bubble's points up) and
// is held back by the air: in proportion to its speed through it for something light
// and slow - a bubble, a gout of foam - and to the square of that speed for anything
// small and fast - a dart, a nail, a hook. Both act on its speed through the air, not
// over the ground, and the air moves: so a bubble goes where the wind goes, a dart
// drifts a little over a long lob, and a nail hardly notices.
//
// It is integrated in steps of a fixed length, however long the frame was: a shot
// thrown at twenty frames a second lands where the same shot thrown at a hundred and
// forty does.
//
// An aimed shot is solved before it leaves rather than drawn along a made-up curve:
// the launcher finds the elevation and the lead into the wind that put it on the
// mark at the tool's speed, and the shot flies the path that comes out.
//
// Implements: REQ-TOOL-081

import * as THREE from '../vendor/three.module.min.js';

/** What anything solid that is thrown falls at, units a second squared. */
export const GRAVITY = 9;

const STEP = 1 / 120;      // seconds: the length of one step of a flight
const MAX_FLIGHT = 8;      // seconds: a solution that takes longer is none
const ON_MARK = 0.01;      // units: near enough to the mark to call it hit
const TRIES = 16;          // corrections to the aim before giving up on it

/**
 * Moves a shot on by `dt` seconds: `pos` and `vel` in place, through air moving at
 * `wind`. `flight` is the tool's: gravity, drag (per second, on the speed through
 * the air) and cd (per unit, on its square).
 */
export function fly(pos, vel, flight, wind, dt) {
  const n = Math.max(1, Math.ceil(dt / STEP - 1e-9)), h = dt / n;
  const linear = flight.drag || 0, square = flight.cd || 0, g = flight.gravity || 0;
  for (let i = 0; i < n; i++) {
    const rx = vel.x - wind.x, ry = vel.y - wind.y, rz = vel.z - wind.z;
    const k = linear + square * Math.hypot(rx, ry, rz);
    vel.x -= k * rx * h;
    vel.y -= (k * ry + g) * h;
    vel.z -= k * rz * h;
    pos.x += vel.x * h;
    pos.y += vel.y * h;
    pos.z += vel.z * h;
  }
}

/**
 * The flight of a shot from `start` that lands on `to`, at the tool's speed, through
 * `wind` (held as it is now - it changes over seconds, a shot is over in one or two):
 * {path: the positions every STEP, T: how long it takes, vel: how it arrives}, or null
 * when the tool cannot throw that far. The lower of the two arcs that reach is the
 * one taken, which is how anybody aims.
 */
export function aim(start, to, flight, wind) {
  const dx = to.x - start.x, dy = to.y - start.y, dz = to.z - start.z;
  const D = Math.hypot(dx, dz), v = flight.speed;
  if (D < 0.3) return straight(start, to, v);
  const ux = dx / D, uz = dz / D; // along the shot, flat
  const g = flight.gravity || 0;
  // The vacuum's answer to start from: right without air or wind, and near enough
  // with them that a few corrections finish it.
  const disc = v ** 4 - g * (g * D * D + 2 * dy * v * v);
  let pitch = g > 0 && disc >= 0 ? Math.atan((v * v - Math.sqrt(disc)) / (g * D)) : Math.atan2(dy, D);
  let side = 0; // the lead into the wind, radians off the line to the mark
  let last = null;
  for (let i = 0; i < TRIES; i++) {
    const shot = trace(start, pitch, Math.atan2(ux, uz) + side, v, flight, wind, D, ux, uz);
    if (!shot) { // fell short: higher, while there is any higher to go
      if (pitch > 1.3) return null;
      pitch = Math.min(1.35, pitch + 0.12);
      last = null;
      continue;
    }
    const end = shot.path.at(-1);
    const high = end.y - to.y;
    const wide = (end.x - to.x) * uz - (end.z - to.z) * ux;
    if (Math.abs(high) < ON_MARK && Math.abs(wide) < ON_MARK) {
      end.copy(to);
      return shot;
    }
    // Up or down by the secant through the last two tries; the first goes by the
    // geometry alone. Across by the angle the miss subtends.
    const slope = last && Math.abs(pitch - last.pitch) > 1e-6 ? (high - last.high) / (pitch - last.pitch) : D / Math.cos(pitch) ** 2;
    last = { pitch, high };
    pitch -= high / (Math.abs(slope) > 1e-3 ? slope : D);
    pitch = Math.max(-1.45, Math.min(1.35, pitch));
    side -= wide / D;
  }
  return null;
}

// A shot thrown at `pitch` and `heading` until it has come D along (ux, uz) - the
// path, ending exactly there - or null if it falls into the water or runs out of time
// first.
function trace(start, pitch, heading, v, flight, wind, D, ux, uz) {
  const pos = start.clone();
  const vel = new THREE.Vector3(Math.sin(heading) * Math.cos(pitch), Math.sin(pitch), Math.cos(heading) * Math.cos(pitch)).multiplyScalar(v);
  const path = [pos.clone()];
  const along = p => (p.x - start.x) * ux + (p.z - start.z) * uz;
  for (let t = 0; t < MAX_FLIGHT; t += STEP) {
    const before = pos.clone(), was = along(before);
    fly(pos, vel, flight, wind, STEP);
    const now = along(pos);
    if (now >= D) {
      const u = (D - was) / Math.max(1e-9, now - was);
      path.push(before.lerp(pos, u));
      return { path, T: t + STEP * u, vel: vel.clone() };
    }
    if (now <= was && t > 0.5) return null; // blown back, or stopped dead in the air
    path.push(pos.clone());
    if (pos.y < start.y - 60) return null;
  }
  return null;
}

// Straight up or down, or so close it makes no difference: a line.
function straight(start, to, v) {
  const T = Math.max(STEP, start.distanceTo(to) / v);
  const n = Math.max(1, Math.ceil(T / STEP));
  const path = Array.from({ length: n + 1 }, (_, i) => start.clone().lerp(to, i / n));
  return { path, T, vel: to.clone().sub(start).setLength(v) };
}

/** Where on `path` a shot is `t` seconds into a flight of `T`. */
export function along(path, T, t, out) {
  const s = Math.min(1, t / T) * (path.length - 1);
  const i = Math.min(path.length - 2, Math.floor(s));
  if (i < 0) return out.copy(path[0]);
  return out.copy(path[i]).lerp(path[i + 1], s - i);
}

/**
 * The wind over the map. It has a prevailing direction that wanders over minutes, a
 * strength that swells and slackens over tens of seconds, and gusts over a few;
 * nothing about it is random from one frame to the next, so a shot sees the air it
 * was thrown into. `at` takes seconds and a vector to fill, units a second along x
 * and z; the air does not move up or down.
 */
export class Breeze {
  constructor(seed = Math.random() * 100) {
    this.seed = seed;
  }

  at(t, out = new THREE.Vector3()) {
    const s = this.seed;
    const heading = s + 0.9 * Math.sin(t * 0.011 + s) + 0.35 * Math.sin(t * 0.037 + 2 * s);
    const swell = 0.65 + 0.35 * Math.sin(t * 0.045 + 3 * s);
    const gust = 1 + 0.45 * Math.max(0, Math.sin(t * 0.61 + s) * Math.sin(t * 1.37 + 4 * s));
    const strength = BREEZE * swell * gust;
    return out.set(Math.sin(heading) * strength, 0, Math.cos(heading) * strength);
  }
}

/** The wind's mean strength, units a second: a little under a walker's pace. */
export const BREEZE = 2.2;
