// The walker's sense of where things are: the sweep in the corner of the walk HUD -
// the bugs, the fires and the tagged modules around them - and the beacons standing
// over every module tagged. Mixed into Walker (walk.js).

import * as THREE from '../vendor/three.module.min.js';
import { clamp, ease } from '../core/numbers.js';
import { severityColors, rankOf } from '../core/findings.js';

// The bug tracker: how far around the walker it sweeps, and how often it is redrawn.
// A dozen times a second is plenty for something that turns as slowly as a walker.
// The range follows the hunt - a four-file repository and a thousand-file one are both
// worth seeing whole - within these bounds, and eases rather than jumping.
// Implements: REQ-HUNT-025
const RADAR_MIN = 12, RADAR_MAX = 400, RADAR_MS = 85, RADAR_SIZE = 150;
// Closing in: inside this, the sweep stops trying to hold the whole map and draws the
// neighborhood instead, growing by up to RADAR_GROW as it does. The last few steps
// to a bug are the ones worth seeing, and they are the ones a whole-map sweep loses.
// 14 rather than something roomier because a city block is a few units across: any
// wider and a map with bugs on every street is zoomed in the whole time, which is
// the same as not zooming at all.
const RADAR_NEAR = 14, RADAR_GROW = 0.55;
// How long a fire takes to pulse on the sweep, in milliseconds. Slow enough to read
// as breathing rather than blinking: a blink is an alarm and there is nothing sudden
// about a fire, which is already alight and will still be alight in a second.
const RADAR_PULSE = 1500;
// What a fire is drawn in on the sweep. Not a severity color: severity is what the
// scanners said, and every fire here is the same thing whatever they rated it - a
// vulnerability somebody can reach. It is the color of the flames instead, so the
// dot and the thing it stands for are plainly the same.
const FIRE_DOT = '#f26a1b';
// How long the range and the dial take to settle, in seconds. Eased by elapsed time
// rather than per redraw, so the growth is the same on a slow frame as on a fast one.
const RADAR_RANGE_TAU = 0.5, RADAR_ZOOM_TAU = 0.55;

// What a beacon is over a module the scanners had nothing to say about; anything they
// did have something to say about wears the color of the worst of it instead.
const BEACON = '#ff8a1f';
let beaconParts = null; // the diamond and its beam, shared by every beacon

export const tracker = {
  // Implements: REQ-HUNT-021, REQ-HUNT-022, REQ-HUNT-023, REQ-HUNT-024, REQ-HUNT-025, REQ-HUNT-026
  drawRadar(now, deltaTime) {
    const box = this.hud.querySelector('.w-radar');
    // A map whose every advisory is reachable has no bugs walking it at all, and the
    // sweep is worth more there than anywhere: everything on it is on fire.
    const alight = this.burning();
    if (!this.bugs?.bugs.length && !alight.length) {
      box.hidden = true;
      return;
    }
    box.hidden = false;
    const canvas = box.querySelector('canvas');
    const { x: px, z: pz, yaw } = this.p;
    const live = this.bugs?.bugs.filter(b => !b.caught) || [];
    const closing = this.zoomRadar(canvas, [...live.map(b => b.position), ...alight], deltaTime);

    // The contents turn as slowly as a walker does, so they are repainted a dozen
    // times a second; the scale follows every frame.
    if (now - (this.radarAt || 0) < RADAR_MS) return;
    this.radarAt = now;

    const computedStyle = getComputedStyle(this.hud);
    const v = name => computedStyle.getPropertyValue(name).trim();
    // Drawn in CSS pixels of the unscaled dial, whatever the bitmap behind it is.
    const size = RADAR_SIZE;
    const g = canvas.getContext('2d');
    const c = size / 2, R = c - 7;
    const unit = canvas.width / size;
    g.setTransform(unit, 0, 0, unit, 0, 0);
    g.clearRect(0, 0, size, size);
    const k = R / this.radarRange;
    drawSweep(g, c, R, this.scene.walkCamera, v);

    // Everything is placed in the walker's frame: forward is up.
    const sin = Math.sin(yaw), cos = Math.cos(yaw);
    const place = (x, z) => {
      const dx = x - px, dz = z - pz;
      const u = dx * cos - dz * sin, f = -dx * sin - dz * cos;
      const d = Math.hypot(u, f);
      const inside = d * k <= R - 4;
      const scale = inside ? k : (R - 4) / Math.max(d, 1e-6);
      return { x: c + u * scale, y: c - f * scale, d, inside, angle: Math.atan2(u, f) };
    };

    // The severity colors, for the bugs and for the rings around what is tagged.
    const colors = this.bugs?.colors || {};

    // North, so the sweep can be read against the map it came from.
    const n = place(px, pz - this.radarRange * 4);
    g.fillStyle = v('--muted');
    g.font = '600 9px system-ui, sans-serif';
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText('N', n.x, n.y);

    // Modules already tagged: where the hunt has been, each ring in the color of the
    // worst thing found on it, the way its beacon is.
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      const q = place(b.x, b.z);
      if (!q.inside) continue;
      g.strokeStyle = colors[this.severityOf(b.node)] || v('--muted');
      g.beginPath();
      g.arc(q.x, q.y, 2.6, 0, Math.PI * 2);
      g.stroke();
    }

    const nearest = drawBugs(g, live, place, colors, v);
    drawFires(g, alight, place, now);

    // How far the sweep reaches, so a dot's distance can be read off it.
    g.fillStyle = v('--muted');
    g.font = '9px system-ui, sans-serif';
    g.textAlign = 'right';
    g.textBaseline = 'bottom';
    g.fillText(`${Math.round(this.radarRange)}`, size - 2, size - 1);

    // The walker, facing up.
    g.fillStyle = v('--text');
    g.beginPath();
    g.moveTo(c, c - 5);
    g.lineTo(c + 3.6, c + 4);
    g.lineTo(c - 3.6, c + 4);
    g.closePath();
    g.fill();

    // Close in, the nearest bug is marked as well as drawn: a ring around it, so the
    // one being walked up to is not one dot among several.
    if (nearest && closing > 0.25) {
      const q = place(nearest.bug.position.x, nearest.bug.position.z);
      if (q.inside) {
        g.strokeStyle = colors[nearest.bug.f.severity] || v('--text');
        g.lineWidth = 1.5;
        g.beginPath();
        g.arc(q.x, q.y, 6.5, 0, Math.PI * 2);
        g.stroke();
      }
    }

    const label = this.hud.querySelector('.w-nearest');
    label.textContent = radarLabel(alight, nearest, px, pz, this.bugs?.counts || { caught: 0, total: 0 });
  },

  /**
   * Eases the sweep's range and the dial's size towards the marks at `marks`, and
   * returns how far it is closing in on the nearest, 0 to 1.
   *
   * The range fits whatever is still out there, so the sweep is never all center
   * dot or all rim arrows. Once one is close, though, holding the whole map is the
   * wrong thing to hold: the range pulls in to the neighborhood and the dial grows
   * to meet it, which is the difference between knowing a bug is somewhere ahead
   * and seeing which side of the building it is on.
   */
  zoomRadar(canvas, marks, deltaTime) {
    const { x: px, z: pz } = this.p;
    let far = RADAR_MIN, near = Infinity;
    for (const at of marks) {
      const d = Math.hypot(at.x - px, at.z - pz);
      far = Math.max(far, d);
      near = Math.min(near, d);
    }
    const closing = near < RADAR_NEAR ? 1 - near / RADAR_NEAR : 0;
    this.radarNear = near; // what the sweep thinks it is closing on, for the tests

    // Both the range and the dial ease towards where they are going, and both ease
    // by elapsed time rather than by redraw, so the approach looks the same whatever
    // the frame rate is and whatever the throttle in drawRadar decides.
    const want = closing
      ? clamp(Math.max(near * 2.4, RADAR_MIN), RADAR_MIN, RADAR_NEAR * 2.4)
      : clamp(far * 1.2, RADAR_MIN, RADAR_MAX);
    this.radarRange = ease(this.radarRange, want, RADAR_RANGE_TAU, deltaTime);
    this.radarZoom = ease(this.radarZoom, 1 + closing * RADAR_GROW, RADAR_ZOOM_TAU, deltaTime);
    // The dial grows by transform rather than by resizing the canvas: a scale is
    // sub-pixel and costs the compositor alone, where a resize rounded to whole
    // pixels grew in visible steps, threw the drawing away and reallocated the
    // bitmap on the way. The bitmap is therefore made once, at the size the dial
    // reaches when it is fully grown, so growing into it stays sharp.
    canvas.style.transform = `scale(${this.radarZoom.toFixed(3)})`;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const pixels = Math.round(RADAR_SIZE * (1 + RADAR_GROW) * dpr);
    if (canvas.width !== pixels) canvas.width = canvas.height = pixels;
    return closing;
  },

  /**
   * A beacon stands over every tagged module that has a box: a diamond on a thin light
   * beam, so the hunt's trophies are visible across the city - in the color of the
   * worst finding on it, so what the beam says is not only that the module was tagged
   * but how bad what is in it is. A module the scanners had nothing to say about keeps
   * the plain beacon, which is what a clean module looks like from across the map.
   *
   * Implements: REQ-HUNT-003, REQ-HUNT-004
   */
  drawBeacons() {
    this.beacons.clear();
    if (!this.tagged.size) return;
    beaconParts ||= [
      new THREE.OctahedronGeometry(0.16).scale(1, 1.6, 1).translate(0, 1.75, 0),
      new THREE.CylinderGeometry(0.018, 0.018, 1.5, 6).translate(0, 0.75, 0),
    ];
    const colors = severityColors();
    for (const b of this.boxes) {
      if (b.kind === 'land' || !this.tagged.has(b.node.id)) continue;
      const color = colors[this.severityOf(b.node)] || BEACON;
      for (const geo of beaconParts) {
        const m = new THREE.Mesh(geo, this.beaconMaterial(color));
        m.position.set(b.x, b.y + b.h, b.z);
        m.frustumCulled = false;
        this.beacons.add(m);
      }
    }
  },

  // One material per color, however many beacons there are: seven at the very most,
  // and they are rebuilt every time something is tagged.
  beaconMaterial(color) {
    const beams = this.beams ||= new Map();
    let m = beams.get(color);
    if (!m) {
      m = this.scene.bendable(new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.85 }));
      beams.set(color, m);
    }
    return m;
  },
};

// ------------------------------------------------------------------ the sweep

/**
 * The sweep itself: a disc, range rings, and the wedge the walker is looking into -
 * the horizontal field of view, which is the vertical one widened by the aspect ratio.
 */
function drawSweep(g, c, R, camera, v) {
  g.save();
  g.beginPath();
  g.arc(c, c, R, 0, Math.PI * 2);
  g.fillStyle = v('--surface');
  g.globalAlpha = 0.84;
  g.fill();
  g.globalAlpha = 1;
  g.clip();
  const half = Math.atan(Math.tan((camera.fov * Math.PI / 180) / 2) * (camera.aspect || 1.6));
  g.beginPath();
  g.moveTo(c, c);
  g.arc(c, c, R, -Math.PI / 2 - half, -Math.PI / 2 + half);
  g.closePath();
  g.fillStyle = v('--grid');
  g.globalAlpha = 0.75;
  g.fill();
  g.globalAlpha = 1;
  g.restore();
  g.strokeStyle = v('--border');
  g.lineWidth = 1;
  for (const r of [R, R * 0.66, R * 0.33]) {
    g.beginPath();
    g.arc(c, c, r, 0, Math.PI * 2);
    g.stroke();
  }
}

/** An arrow on the sweep's rim at `q`, pointing the way to what is out of range. */
function rimArrow(g, q, tip, side, alpha = null) {
  g.save();
  g.translate(q.x, q.y);
  g.rotate(q.angle);
  if (alpha !== null) g.globalAlpha = alpha;
  g.beginPath();
  g.moveTo(0, -tip);
  g.lineTo(side, side);
  g.lineTo(-side, side);
  g.closePath();
  g.fill();
  g.restore();
}

/**
 * The bugs, worst drawn last so a critical one is never hidden under a nit; returns
 * the nearest, { d, bug }, or null.
 */
function drawBugs(g, live, place, colors, v) {
  live.sort((a, b) => rankOf(a.f.severity) - rankOf(b.f.severity));
  let nearest = null;
  for (const bug of live) {
    const m = bug.position;
    const q = place(m.x, m.z);
    if (!nearest || q.d < nearest.d) nearest = { d: q.d, bug };
    g.fillStyle = colors[bug.f.severity] || v('--muted');
    if (q.inside) {
      g.beginPath();
      g.arc(q.x, q.y, 3, 0, Math.PI * 2);
      g.fill();
    } else {
      rimArrow(g, q, 4, 3);
    }
  }
  return nearest;
}

/**
 * The fires, last of all and over everything else on the sweep.
 *
 * They pulse, and nothing else on the dial does. A bug is a still dot and a tagged
 * module a still ring, because neither is going anywhere: a bug walks its lap and
 * waits to be caught, and a module stays tagged. A fire is the one mark here that
 * is getting worse while it is being looked at, and the one worth turning round
 * for - so it is the one that moves. What pulses is a ring thrown off the dot and
 * fading as it widens, which is a thing spreading, drawn small.
 */
function drawFires(g, alight, place, now) {
  for (const f of alight) {
    const q = place(f.x, f.z);
    const beat = ((now % RADAR_PULSE) / RADAR_PULSE + f.x * 0.11 + f.z * 0.07) % 1;
    g.fillStyle = FIRE_DOT;
    g.strokeStyle = FIRE_DOT;
    if (q.inside) {
      // The ring first, so the dot it comes off stays solid over it.
      g.save();
      g.globalAlpha = (1 - beat) * 0.7 * (0.4 + f.heat * 0.6);
      g.lineWidth = 1.5;
      g.beginPath();
      g.arc(q.x, q.y, 3 + beat * 7, 0, Math.PI * 2);
      g.stroke();
      g.restore();
      g.beginPath();
      g.arc(q.x, q.y, 2.6 + f.heat * 1.4, 0, Math.PI * 2);
      g.fill();
    } else {
      // Out of range, a fire gets the same rim arrow a bug does, and the same pulse
      // with it: a district alight across the map is worth walking towards, and
      // saying so is most of what the sweep is for.
      rimArrow(g, q, 5, 3.4, 0.55 + (1 - beat) * 0.45);
    }
  }
}

/**
 * The line under the sweep. What is alight comes first, whatever else the sweep is
 * showing: it is the only thing on there that gets worse for being left.
 */
function radarLabel(alight, nearest, px, pz, { caught, total }) {
  const fire = alight.map(f => ({ f, d: Math.hypot(f.x - px, f.z - pz) }))
    .sort((a, b) => a.d - b.d)[0];
  if (fire) return `${alight.length} alight · nearest ${Math.round(fire.d)} away · ${fire.f.node.name}`;
  if (nearest) return `nearest ${Math.round(nearest.d)} away · ${nearest.bug.f.severity}: ${nearest.bug.f.title}`;
  return caught ? `all ${total} bugs caught` : '';
}
