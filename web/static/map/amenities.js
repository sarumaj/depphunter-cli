// What a park holds besides its trees, in each style: pitches and courts to play ball
// on, playgrounds to ride, and on a board and in the galaxy a few things that only
// stand there. Each fills one square of lawn between the park's gravel paths
// (city.js plantParks), modeled along x about its middle with the ground at 0, and
// says how much ground it covers as modeled (size, [along x, along z]), how much it is
// enlarged to stand true to the walker - who is about half a unit tall, so a unit is
// some three and a half meters (scale) - how often it is chosen (weight), and where it
// stands in the walker's way as modeled: posts, [x, z, radius].
//
// What moves is apart from what stands: rigs, each a geometry turned about its `at`,
// so the one being ridden can be posed while the rest stay drawn as instances. How an
// amenity is played is `play`, in the same model coordinates: what can be ridden and
// how (swing, spin, rock, slide), where its ball is put out, and the hoops, goals and
// nets the ball is played at (walk/play.js).
//
// Implements: REQ-CITY-039, REQ-CITY-040

import * as THREE from '../vendor/three.module.min.js';
import { merge, shaded, painted, patch, stripe, outline, ring, post, bar, block, lit } from './shapes.js';

// ------------------------------------------------------------------ pieces

const piece = (head, more = {}) => ({ head, rigs: [], play: [], posts: [], glows: [], ...more });

const moved = (p, dx, dz) => [p[0] + dx, p[1], p[2] + dz];
const isPoint = v => Array.isArray(v) && v.length === 3 && v.every(n => typeof n === 'number');

// A play entry moved by dx, dz, its rig numbered among the amenity's from `base`. A
// seat is where on its rig the rider sits, and moves with the rig.
function shifted(entry, dx, dz, base) {
  const out = {};
  for (const [key, v] of Object.entries(entry)) {
    if (key === 'rig') out.rig = v + base;
    else if (key === 'seat') out.seat = v;
    else if (key === 'area') out.area = [v[0] + dx, v[1] + dz, v[2] + dx, v[3] + dz];
    else if (isPoint(v)) out[key] = moved(v, dx, dz);
    else if (Array.isArray(v) && v.length && v.every(isPoint)) out[key] = v.map(p => moved(p, dx, dz));
    else if (v && typeof v === 'object' && !Array.isArray(v)) out[key] = shifted(v, dx, dz, base);
    else out[key] = v;
  }
  return out;
}

/** One amenity from [piece, dx, dz]s. `glow` lights the pieces' glows: { color, shells: [[radius, strength]] }. */
function amenity(parts, { size, scale, weight = 1, glow = null }) {
  const head = [], rigs = [], play = [], posts = [], glows = [];
  for (const [p, dx = 0, dz = 0] of parts) {
    const base = rigs.length;
    head.push(...p.head.map(g => g.clone().translate(dx, 0, dz)));
    rigs.push(...p.rigs.map(r => ({ ...r, at: moved(r.at, dx, dz) })));
    play.push(...p.play.map(e => shifted(e, dx, dz, base)));
    posts.push(...p.posts.map(([x, z, r]) => [x + dx, z + dz, r]));
    glows.push(...p.glows.map(g => moved(g, dx, dz)));
  }
  return {
    head: merge(head), rigs, play, posts, size, scale, weight,
    glow: glow && glows.length ? { color: glow.color, shells: lit(glows, glow.shells) } : null,
  };
}

// The edges of the upper half of a geodesic sphere, for a climbing dome.
function domeEdges(r) {
  const position = new THREE.IcosahedronGeometry(r, 1).getAttribute('position');
  const seen = new Set(), edges = [];
  const key = p => p.map(n => n.toFixed(4)).join();
  for (let i = 0; i < position.count; i += 3) {
    const tri = [0, 1, 2].map(k => [position.getX(i + k), position.getY(i + k), position.getZ(i + k)]);
    for (const [a, b] of [[tri[0], tri[1]], [tri[1], tri[2]], [tri[2], tri[0]]]) {
      if (a[1] < -1e-3 || b[1] < -1e-3) continue;
      const k = [key(a), key(b)].sort().join('|');
      if (!seen.has(k)) { seen.add(k); edges.push([a, b]); }
    }
  }
  return edges;
}

// ------------------------------------------------------------------ playground pieces

// A swing set: an A-frame at each end of a beam along z, a seat hanging from the beam
// for each of `seats` (their z), swinging along x.
function swings(kit, { seats = [-0.13, 0.13], h = 0.32, seatY = 0.07, nest = false } = {}) {
  const z0 = Math.min(...seats) - (nest ? 0.2 : 0.16), z1 = Math.max(...seats) + (nest ? 0.2 : 0.16), length = h - seatY;
  const head = [];
  for (const z of [z0, z1]) head.push(kit.leg([-0.1, 0, z], [0, h, z]), kit.leg([0.1, 0, z], [0, h, z]));
  head.push(kit.beam([0, h, z0 - 0.02], [0, h, z1 + 0.02]));
  const p = piece(head, { posts: [[-0.1, z0, 0.03], [0.1, z0, 0.03], [-0.1, z1, 0.03], [0.1, z1, 0.03]] });
  for (const z of seats) {
    const geo = nest
      ? merge([...[[-1, -1], [-1, 1], [1, -1], [1, 1]].map(([sx, sz]) => kit.chain([0, 0, sz * 0.06], [sx * 0.06, -length, sz * 0.05])),
        kit.nest(-length)])
      : merge([kit.chain([0, 0, -0.04], [0, -length, -0.04]), kit.chain([0, 0, 0.04], [0, -length, 0.04]), kit.seat(-length)]);
    p.rigs.push({ geo, at: [0, h, z], axis: 'z' });
    p.play.push({ ride: 'swing', rig: p.rigs.length - 1, pivot: [0, h, z], length });
  }
  p.glows.push(...(kit.glowsOn?.swings?.(h, z0, z1) || []));
  return p;
}

// A slide down along +x: the ladder up, the platform, and the chute; `tower` puts a
// roof over a taller platform.
function slide(kit, { h = 0.24, run = 0.38, tower = false } = {}) {
  const slope = Math.atan2(h - 0.02, run), long = Math.hypot(run, h - 0.02);
  const head = [
    kit.leg([-0.1, 0, -0.05], [-0.01, h, -0.05]), kit.leg([-0.1, 0, 0.05], [-0.01, h, 0.05]),
    ...Array.from({ length: Math.floor(h / 0.06) }, (_, i) => {
      const y = 0.06 * (i + 1), x = -0.1 + 0.09 * y / h;
      return kit.rung([x, y, -0.05], [x, y, 0.05]);
    }),
    ...[[0, -0.06], [0, 0.06], [0.12, -0.06], [0.12, 0.06]].map(([x, z]) => kit.leg([x, 0, z], [x, h, z])),
    block(0.14, 0.015, 0.14, 0.06, h, 0, kit.deck),
    painted(new THREE.BoxGeometry(long, 0.012, 0.11).rotateZ(-slope).translate(0.12 + run / 2, (h + 0.02) / 2 + 0.006, 0), kit.slide),
    ...[-0.056, 0.056].map(z => kit.rail([0.12, h + 0.045, z], [0.12 + run, 0.06, z])),
  ];
  if (tower) {
    head.push(...[[0, -0.06], [0, 0.06], [0.12, -0.06], [0.12, 0.06]].map(([x, z]) => kit.leg([x, h, z], [x, h + 0.2, z])));
    head.push(kit.roof(0.06, h + 0.2));
  }
  const p = piece(head, { posts: [[0.06, 0, 0.09]] });
  p.play.push({ ride: 'slide', path: [[-0.14, 0, 0], [-0.03, h + 0.015, 0], [0.08, h + 0.015, 0], [0.13, h + 0.012, 0], [0.12 + run, 0.03, 0], [0.24 + run, 0, 0]] });
  p.glows.push(...(kit.glowsOn?.slide?.(h, run) || []));
  return p;
}

// A roundabout: a deck that turns on a base, with a pole in the middle and bars to
// hold on to.
function carousel(kit, { r = 0.22 } = {}) {
  const head = [painted(new THREE.CylinderGeometry(r * 1.05, r * 1.08, 0.02, 20).translate(0, 0.01, 0), kit.base)];
  const turning = merge([
    kit.platform(r),
    post(0, 0, 0.18, 0.012, kit.frame, 0.02),
    ...Array.from({ length: 4 }, (_, i) => {
      const a = i * Math.PI / 2, x = Math.cos(a), z = Math.sin(a);
      return merge([kit.rail([x * 0.02, 0.15, z * 0.02], [x * r * 0.85, 0.15, z * r * 0.85]),
        kit.rail([x * r * 0.85, 0.055, z * r * 0.85], [x * r * 0.85, 0.15, z * r * 0.85])]);
    }),
    kit.spire?.() || shaded(new THREE.SphereGeometry(0.02, 8, 6).translate(0, 0.2, 0)),
  ]);
  const p = piece(head, { posts: [[0, 0, 0.03]] });
  p.rigs.push({ geo: turning, at: [0, 0, 0], axis: 'y' });
  p.play.push({ ride: 'spin', rig: 0, pivot: [0, 0, 0], r: r * 0.68, floor: 0.055 });
  p.glows.push(...(kit.glowsOn?.carousel?.() || []));
  return p;
}

// A seesaw along x, rocking about z on its middle; one seat at each end.
function seesaw(kit, { length = 0.5 } = {}) {
  const half = length / 2;
  const plank = merge([
    painted(new THREE.BoxGeometry(length, 0.012, 0.05).translate(0, 0.006, 0), kit.seatColor),
    ...[-1, 1].map(s => merge([kit.rail([s * (half - 0.07), 0.012, -0.02], [s * (half - 0.07), 0.06, -0.02]),
      kit.rail([s * (half - 0.07), 0.012, 0.02], [s * (half - 0.07), 0.06, 0.02]),
      kit.rail([s * (half - 0.07), 0.06, -0.025], [s * (half - 0.07), 0.06, 0.025])])),
  ]);
  const p = piece([kit.fulcrum()], { posts: [[0, 0, 0.04]] });
  p.rigs.push({ geo: plank, at: [0, 0.055, 0], axis: 'z', rest: 0.12 });
  for (const s of [-1, 1]) p.play.push({ ride: 'rock', rig: 0, pivot: [0, 0.055, 0], seat: [s * (half - 0.035), 0.012, 0], reach: 0.2 });
  return p;
}

// A rider on a spring, rocking back and forth along x.
function springRider(kit) {
  const coil = merge(Array.from({ length: 6 }, (_, i) => painted(new THREE.TorusGeometry(0.025, 0.005, 4, 10).rotateX(Math.PI / 2).translate(0, 0.012 + i * 0.012, 0), kit.spring)));
  const p = piece([painted(new THREE.CylinderGeometry(0.05, 0.05, 0.008, 12).translate(0, 0.004, 0), kit.frame)], { posts: [[0, 0, 0.03]] });
  p.rigs.push({ geo: merge([coil, kit.rider()]), at: [0, 0, 0], axis: 'z' });
  p.play.push({ ride: 'rock', rig: 0, pivot: [0, 0, 0], seat: [0, 0.115, 0], reach: 0.3 });
  return p;
}

// A climbing dome, to stand round.
function dome(kit, { r = 0.2 } = {}) {
  const p = piece(domeEdges(r).map(([a, b]) => bar(a, b, 0.006, kit.domeColor || kit.frame)), { posts: [[0, 0, r * 0.95]] });
  p.glows.push(...(kit.glowsOn?.dome?.(r) || []));
  return p;
}

const ground = (kit, w, d) => piece([patch(w, d, kit.ground), outline(w, d, kit.edge, 0, 0, 0.03)]);

// ------------------------------------------------------------------ pitches and courts

// A five-a-side pitch along x with a goal at each end.
function pitch(kit) {
  const p = piece([
    patch(1.78, 1.18, kit.turf), patch(0.3, 1.18, kit.turfLight, -0.6), patch(0.3, 1.18, kit.turfLight, 0), patch(0.3, 1.18, kit.turfLight, 0.6),
    outline(1.7, 1.1, kit.line), stripe(0, -0.55, 0, 0.55, kit.line), ring(0.15, kit.line),
    outline(0.24, 0.5, kit.line, -0.73), outline(0.24, 0.5, kit.line, 0.73), kit.goal(-1), kit.goal(1),
  ], { posts: [[-0.85, -0.13, 0.03], [-0.85, 0.13, 0.03], [0.85, -0.13, 0.03], [0.85, 0.13, 0.03]] });
  p.play.push({ ball: 'soccer', at: [0, 0, 0], area: [-0.85, -0.55, 0.85, 0.55] });
  for (const side of [-1, 1]) p.play.push({ goal: side, from: [side * 0.85, 0, -0.13], to: [side * 0.85, 0.12, 0.13] });
  p.glows.push(...(kit.glowsOn?.goals?.() || []));
  return p;
}

// A basketball court along x with a hoop at each end, its board facing in.
function court(kit) {
  const p = piece([
    patch(1.5, 0.98, kit.court), patch(0.26, 0.28, kit.key, -0.6), patch(0.26, 0.28, kit.key, 0.6),
    outline(1.36, 0.86, kit.line), stripe(0, -0.43, 0, 0.43, kit.line), ring(0.13, kit.line),
    outline(0.26, 0.28, kit.line, -0.55), outline(0.26, 0.28, kit.line, 0.55),
    ring(0.36, kit.line, -0.68, 0, -Math.PI / 2, Math.PI), ring(0.36, kit.line, 0.68, 0, Math.PI / 2, Math.PI),
    kit.hoop(-1), kit.hoop(1),
  ], { posts: [[-0.68, 0, 0.03], [0.68, 0, 0.03]] });
  p.play.push({ ball: 'basket', at: [0, 0, 0], area: [-0.68, -0.43, 0.68, 0.43] });
  for (const side of [-1, 1]) {
    p.play.push({ hoop: [side * 0.565, 0.25, 0], r: 0.028, board: { from: [side * 0.6 - 0.006, 0.24, -0.085], to: [side * 0.6 + 0.006, 0.34, 0.085] } });
  }
  p.glows.push(...(kit.glowsOn?.hoops?.() || []));
  return p;
}

// A volleyball court along x, the net across its middle along z.
const NET_LOW = 0.14, NET_TOP = 0.231, NET_SIDE = 0.5;
function volley(kit) {
  const strings = [
    ...Array.from({ length: 6 }, (_, i) => bar([0, NET_LOW + i * 0.015, -NET_SIDE + 0.02], [0, NET_LOW + i * 0.015, NET_SIDE - 0.02], 0.0012, kit.net)),
    ...Array.from({ length: 31 }, (_, i) => {
      const z = -NET_SIDE + 0.02 + i * (2 * NET_SIDE - 0.04) / 30;
      return bar([0, NET_LOW, z], [0, NET_TOP, z], 0.0012, kit.net);
    }),
  ];
  const p = piece([
    patch(2.1, 1.25, kit.sand), outline(1.71, 0.86, kit.tape, 0, 0, 0.016), stripe(0, -0.43, 0, 0.43, kit.tape, 0.016),
    ...[-1, 1].map(s => kit.pole(s * NET_SIDE, NET_TOP + 0.02)),
    ...strings, kit.netTop([0, NET_TOP, -NET_SIDE], [0, NET_TOP, NET_SIDE]),
    ...[-1, 1].map(s => bar([0, NET_LOW, s * 0.43], [0, NET_TOP + 0.06, s * 0.43], 0.002, kit.antenna)),
  ], { posts: [[0, -NET_SIDE, 0.02], [0, NET_SIDE, 0.02]] });
  p.play.push({ ball: 'volley', at: [-0.6, 0, 0], area: [-0.855, -0.43, 0.855, 0.43] });
  p.play.push({ net: true, from: [0, NET_LOW, -NET_SIDE], to: [0, NET_TOP, NET_SIDE] });
  p.glows.push(...(kit.glowsOn?.net?.(NET_TOP, NET_SIDE) || []));
  return p;
}

// ------------------------------------------------------------------ the styles' kits

// What each style builds them out of: the same shapes for the same games, in its own
// materials - painted steel and sand in a city; standoffs, wire and chips on a board;
// dark alloy and light in the galaxy.
const CITY = {
  frame: '#2f6fb3', base: '#7d848b', deck: '#c23b2c', slide: '#e3b324', spring: '#3b4148', seatColor: '#c23b2c',
  ground: '#d9c28c', edge: '#8a6d45', line: '#f4f4ee',
  turf: '#4c9a42', turfLight: '#57a64b', court: '#b4573c', key: '#d07a52', sand: '#e2cf9a', tape: '#2f6fb3', net: '#2b2b2b', antenna: '#d8433a',
  leg: (a, b) => bar(a, b, 0.009, CITY.frame),
  beam: (a, b) => bar(a, b, 0.009, CITY.frame),
  rung: (a, b) => bar(a, b, 0.004, CITY.slide),
  rail: (a, b) => bar(a, b, 0.004, CITY.frame),
  chain: (a, b) => bar(a, b, 0.002, '#8d9196'),
  seat: y => block(0.06, 0.012, 0.1, 0, y - 0.006, 0, CITY.seatColor),
  nest: y => painted(new THREE.TorusGeometry(0.06, 0.012, 6, 16).rotateX(Math.PI / 2).translate(0, y, 0), '#d8433a'),
  roof: (x, y) => painted(new THREE.ConeGeometry(0.12, 0.08, 4).rotateY(Math.PI / 4).translate(x, y + 0.04, 0), '#d8433a'),
  platform: r => painted(new THREE.CylinderGeometry(r, r, 0.035, 20).translate(0, 0.0375, 0), '#d8433a', 0.05),
  fulcrum: () => block(0.04, 0.05, 0.04, 0, 0, 0, '#3b4148'),
  rider: () => merge([
    painted(new THREE.SphereGeometry(0.045, 10, 6).scale(1.5, 0.8, 0.8).translate(0, 0.1, 0), '#e3b324'),
    painted(new THREE.SphereGeometry(0.03, 8, 6).translate(0.06, 0.14, 0), '#e3b324'),
    bar([0.045, 0.13, -0.03], [0.045, 0.13, 0.03], 0.004, '#3b4148'),
  ]),
  goal: side => {
    const x = side * 0.85, back = side * 0.95, white = '#f2f2ee', net = '#c9ccd0';
    return merge([
      post(x, -0.13, 0.12, 0.007, white), post(x, 0.13, 0.12, 0.007, white), bar([x, 0.12, -0.13], [x, 0.12, 0.13], 0.007, white),
      bar([x, 0.12, -0.13], [back, 0, -0.13], 0.004, net), bar([x, 0.12, 0.13], [back, 0, 0.13], 0.004, net),
      painted(new THREE.PlaneGeometry(0.26, Math.hypot(0.1, 0.12)).rotateY(Math.PI / 2)
        .rotateZ(side * Math.atan2(0.1, 0.12)).translate((x + back) / 2, 0.06, 0), net),
    ]);
  },
  hoop: side => {
    const x = side * 0.68;
    return merge([
      post(x, 0, 0.3, 0.01, '#3b4148'), bar([x, 0.28, 0], [side * 0.6, 0.28, 0], 0.007, '#3b4148'),
      block(0.012, 0.1, 0.17, side * 0.6, 0.24, 0, '#f4f4f0'),
      painted(new THREE.TorusGeometry(0.028, 0.004, 4, 14).rotateX(Math.PI / 2).translate(side * 0.565, 0.25, 0), '#e06a1c'),
    ]);
  },
  pole: (z, h) => post(0, z, h, 0.008, '#e8e8e4'),
  netTop: (a, b) => bar(a, b, 0.004, '#f4f4f0'),
};

const CIRCUIT = {
  frame: '#c8a24a', base: '#1d1f22', deck: '#1f6b3a', slide: '#a9afb5', spring: '#c47a3a', seatColor: '#c0392b',
  ground: '#c58a4a', edge: '#e8ecef', line: '#e8ecef',
  turf: '#b9783f', turfLight: '#c58a4a', court: '#25603f', key: '#2e7a4f', sand: '#1f5c38', tape: '#e8ecef', net: '#8d939a', antenna: '#c0392b',
  leg: (a, b) => painted(hexBar(a, b, 0.011), CIRCUIT.frame),
  beam: (a, b) => bar(a, b, 0.007, '#cfd5db'),
  rung: (a, b) => bar(a, b, 0.004, '#d9b04c'),
  rail: (a, b) => bar(a, b, 0.004, '#cfd5db'),
  chain: (a, b) => bar(a, b, 0.002, '#c47a3a'),
  // A chip on its legs for a seat.
  seat: y => merge([block(0.06, 0.012, 0.1, 0, y - 0.006, 0, '#1d1f22'),
    ...[-0.03, -0.01, 0.01, 0.03].flatMap(z => [-1, 1].map(s => block(0.008, 0.004, 0.006, s * 0.033, y - 0.004, z, '#cfd5db')))]),
  nest: y => painted(new THREE.TorusGeometry(0.06, 0.012, 6, 16).rotateX(Math.PI / 2).translate(0, y, 0), '#1d1f22'),
  roof: (x, y) => block(0.16, 0.03, 0.16, x, y, 0, '#1d1f22'),
  // A fan: its blades are the deck.
  platform: r => merge([
    painted(new THREE.CylinderGeometry(0.06, 0.06, 0.04, 16).translate(0, 0.035, 0), '#2a2d31'),
    ...Array.from({ length: 7 }, (_, i) => painted(new THREE.BoxGeometry(r - 0.05, 0.01, 0.07)
      .rotateX(0.35).translate((r + 0.05) / 2, 0.045, 0).rotateY(i * Math.PI * 2 / 7), '#3a3f45')),
    painted(new THREE.CylinderGeometry(0.045, 0.045, 0.005, 16).translate(0, 0.057, 0), '#c8ced4'),
  ]),
  fulcrum: () => merge([block(0.08, 0.04, 0.06, 0, 0, 0, '#1d1f22'), ...[-0.025, 0.025].map(x => block(0.008, 0.012, 0.07, x, 0, 0, '#cfd5db'))]),
  // A coil on a chip: an inductor ridden as a spring.
  rider: () => merge([block(0.14, 0.03, 0.08, 0, 0.08, 0, '#1d1f22'), block(0.016, 0.008, 0.016, 0.05, 0.11, 0, '#d9b04c'),
    bar([0.05, 0.13, -0.03], [0.05, 0.13, 0.03], 0.004, '#cfd5db'), bar([0.05, 0.11, -0.03], [0.05, 0.13, -0.03], 0.003, '#cfd5db'),
    bar([0.05, 0.11, 0.03], [0.05, 0.13, 0.03], 0.003, '#cfd5db')]),
  // A staple of jumper wire, the net a sheet of shielding mesh.
  goal: side => {
    const x = side * 0.85, back = side * 0.95, wire = '#cfd5db', mesh = '#7d848b';
    return merge([
      bar([x, 0, -0.13], [x, 0.12, -0.13], 0.008, wire), bar([x, 0, 0.13], [x, 0.12, 0.13], 0.008, wire), bar([x, 0.12, -0.13], [x, 0.12, 0.13], 0.008, wire),
      painted(new THREE.PlaneGeometry(0.26, Math.hypot(0.1, 0.12)).rotateY(Math.PI / 2)
        .rotateZ(side * Math.atan2(0.1, 0.12)).translate((x + back) / 2, 0.06, 0), mesh),
      bar([x, 0.12, -0.13], [back, 0, -0.13], 0.004, mesh), bar([x, 0.12, 0.13], [back, 0, 0.13], 0.004, mesh),
    ]);
  },
  // A test point's loop for a rim, on a standoff, before a little board.
  hoop: side => {
    const x = side * 0.68;
    return merge([
      painted(hexBar([x, 0, 0], [x, 0.3, 0], 0.012), '#c8a24a'), bar([x, 0.28, 0], [side * 0.6, 0.28, 0], 0.006, '#cfd5db'),
      block(0.012, 0.1, 0.17, side * 0.6, 0.24, 0, '#1f6b3a'), block(0.013, 0.01, 0.172, side * 0.6, 0.24, 0, '#d9b04c'),
      painted(new THREE.TorusGeometry(0.028, 0.005, 4, 14).rotateX(Math.PI / 2).translate(side * 0.565, 0.25, 0), '#d23c2c'),
    ]);
  },
  pole: (z, h) => merge([block(0.03, 0.03, 0.03, 0, 0, z, '#1d1f22'), block(0.012, h, 0.012, 0, 0, z, '#d9b04c')]),
  // A ribbon cable strung across, its first wire red.
  netTop: (a, b) => merge([bar(a, b, 0.005, '#a9afb5'), bar([a[0], a[1] + 0.006, a[2]], [b[0], b[1] + 0.006, b[2]], 0.002, '#c0392b')]),
};

const GALAXY = {
  frame: '#3a3552', base: '#2a2740', deck: '#4a4466', slide: '#4fd0e8', spring: '#8b86a8', seatColor: '#9a7cff',
  ground: '#1e1b30', edge: '#6ff4ff', line: '#6ff4ff', domeColor: '#6ff4ff',
  turf: '#2a2740', turfLight: '#302c4a', court: '#26233a', key: '#332e50', sand: '#231f36', tape: '#6ff4ff', net: '#6ff4ff', antenna: '#ffd27a',
  leg: (a, b) => bar(a, b, 0.011, GALAXY.frame),
  beam: (a, b) => bar(a, b, 0.009, '#6ff4ff'),
  rung: (a, b) => bar(a, b, 0.004, '#6ff4ff'),
  rail: (a, b) => bar(a, b, 0.004, '#8b86a8'),
  chain: (a, b) => bar(a, b, 0.0025, '#6ff4ff'),
  // A pod for a seat.
  seat: y => painted(new THREE.SphereGeometry(0.05, 10, 6, 0, Math.PI * 2, Math.PI / 2, Math.PI / 2).translate(0, y + 0.04, 0), '#9a7cff'),
  nest: y => painted(new THREE.TorusGeometry(0.06, 0.012, 6, 16).rotateX(Math.PI / 2).translate(0, y, 0), '#9a7cff'),
  roof: (x, y) => painted(new THREE.ConeGeometry(0.03, 0.16, 5).translate(x, y + 0.08, 0), '#9a7cff', 0.15),
  // An orbit: a disc ringed with light.
  platform: r => merge([
    painted(new THREE.CylinderGeometry(r, r, 0.035, 24).translate(0, 0.0375, 0), '#2f2b45'),
    painted(new THREE.TorusGeometry(r * 0.92, 0.006, 4, 32).rotateX(Math.PI / 2).translate(0, 0.056, 0), '#6ff4ff'),
  ]),
  spire: () => painted(new THREE.ConeGeometry(0.03, 0.12, 5).translate(0, 0.25, 0), '#9a7cff', 0.15),
  fulcrum: () => painted(new THREE.ConeGeometry(0.035, 0.06, 5).rotateZ(Math.PI).translate(0, 0.03, 0), '#9a7cff', 0.15),
  rider: () => merge([painted(new THREE.OctahedronGeometry(0.05).scale(1.4, 0.8, 0.8).translate(0, 0.1, 0), '#9a7cff', 0.1),
    bar([0.045, 0.13, -0.03], [0.045, 0.13, 0.03], 0.004, '#6ff4ff')]),
  // An arch, its crossbar a bar of light.
  goal: side => {
    const x = side * 0.85, back = side * 0.95;
    return merge([
      post(x, -0.13, 0.12, 0.009, '#3a3552'), post(x, 0.13, 0.12, 0.009, '#3a3552'), bar([x, 0.12, -0.13], [x, 0.12, 0.13], 0.006, '#6ff4ff'),
      painted(new THREE.PlaneGeometry(0.26, Math.hypot(0.1, 0.12)).rotateY(Math.PI / 2)
        .rotateZ(side * Math.atan2(0.1, 0.12)).translate((x + back) / 2, 0.06, 0), '#2f5b77'),
    ]);
  },
  // A ring of light on a mast, before a pane of dark glass.
  hoop: side => {
    const x = side * 0.68;
    return merge([
      post(x, 0, 0.3, 0.011, '#3a3552'), bar([x, 0.28, 0], [side * 0.6, 0.28, 0], 0.007, '#3a3552'),
      block(0.012, 0.1, 0.17, side * 0.6, 0.24, 0, '#4a4466'),
      painted(new THREE.TorusGeometry(0.028, 0.005, 4, 14).rotateX(Math.PI / 2).translate(side * 0.565, 0.25, 0), '#6ff4ff'),
    ]);
  },
  pole: (z, h) => post(0, z, h, 0.012, '#3a3552'),
  // A curtain of light across, under a bright edge.
  netTop: (a, b) => bar(a, b, 0.005, '#bff9ff'),
  glowsOn: {
    swings: (h, z0, z1) => [[0, h, z0], [0, h, (z0 + z1) / 2], [0, h, z1]],
    carousel: () => [[0, 0.26, 0]],
    dome: r => [[0, r, 0]],
    goals: () => [-1, 1].flatMap(s => [[s * 0.85, 0.12, -0.08], [s * 0.85, 0.12, 0.08]]),
    hoops: () => [-1, 1].map(s => [s * 0.565, 0.25, 0]),
    net: (top, side) => [-0.6, -0.2, 0.2, 0.6].map(z => [0, top, z * side]),
    slide: (h, run) => [[0.12 + run * 0.25, h * 0.8, 0], [0.12 + run * 0.75, h * 0.3, 0]],
  },
};

// A standoff: a hexagonal post between two points.
function hexBar(a, b, r) {
  const from = new THREE.Vector3(...a), to = new THREE.Vector3(...b);
  const geo = new THREE.CylinderGeometry(r, r, from.distanceTo(to), 6);
  geo.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), to.clone().sub(from).normalize()));
  return geo.translate((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2);
}

// ------------------------------------------------------------------ what each style has

// Glows for the galaxy's light: as its lamps' (GLOW in city.js), tighter.
const LIGHT = { color: '#6ff4ff', shells: [[0.035, 0.45], [0.07, 0.15]] };

// Playgrounds, each laid out in its own ground: swings, a slide and a seesaw; a
// roundabout, two spring riders and a climbing dome; a tower slide, a nest swing and
// a seesaw. About 11 m by 10 with swings 2.5 m high.
const playgrounds = (kit, glow = null) => [
  amenity([[ground(kit, 1.5, 1.3)], [swings(kit, { seats: [-0.28, -0.02] }), -0.45, -0.15], [slide(kit), 0.12, -0.3], [seesaw(kit), 0.35, 0.36]],
    { size: [1.5, 1.3], scale: 2.2, weight: 0.5, glow }),
  amenity([[ground(kit, 1.4, 1.2)], [carousel(kit), -0.32, 0.05], [springRider(kit), 0.12, -0.36], [springRider(kit), 0.32, -0.36], [dome(kit), 0.36, 0.26]],
    { size: [1.4, 1.2], scale: 2.2, weight: 0.5, glow }),
  amenity([[ground(kit, 1.5, 1.3)], [slide(kit, { h: 0.34, run: 0.5, tower: true }), -0.5, -0.32], [swings(kit, { seats: [0], h: 0.34, seatY: 0.08, nest: true }), 0.42, 0.15], [seesaw(kit), -0.25, 0.38]],
    { size: [1.5, 1.3], scale: 2.2, weight: 0.5, glow }),
];

// Pitch 30 m by 20 with goals 2 m high; court 18 m by 12 with the rim at 3 m; a
// volleyball court 18 m by 9, its net 2.4 m.
const sports = (kit, glow = null) => [
  amenity([[pitch(kit)]], { size: [1.92, 1.2], scale: 4.5, glow }),
  amenity([[court(kit)]], { size: [1.5, 1.0], scale: 3.5, glow }),
  amenity([[volley(kit)]], { size: [2.1, 1.25], scale: 3, glow }),
];

const PAD_LAMPS = Array.from({ length: 6 }, (_, i) => [Math.cos(i * Math.PI / 3) * 0.72, 0.11, Math.sin(i * Math.PI / 3) * 0.72]);

// The dish: a cap of a sphere DISH_R across, tilted DISH_TILT off the sky, its back
// on the hinge at the mast's top and its feed at the focus, half the radius out.
const DISH_R = 0.34, DISH_TILT = 0.7, DISH_HINGE = [0, 0.4, 0], DISH_OPEN = Math.PI * 0.3;
const DISH_AT = new THREE.Vector3(DISH_HINGE[0] - DISH_R * Math.sin(DISH_TILT), DISH_HINGE[1] + DISH_R * Math.cos(DISH_TILT), DISH_HINGE[2]);
const dishFrame = geo => geo.rotateZ(DISH_TILT).translate(DISH_AT.x, DISH_AT.y, DISH_AT.z);
const dishPoint = (x, y, z) => new THREE.Vector3(x, y, z).applyAxisAngle(new THREE.Vector3(0, 0, 1), DISH_TILT).add(DISH_AT).toArray();
const DISH_FEED = dishPoint(0, -DISH_R / 2, 0);
const DISH_STRUTS = [0, 1, 2].map(i => dishPoint(
  DISH_R * Math.sin(DISH_OPEN) * Math.cos(i * Math.PI * 2 / 3), -DISH_R * Math.cos(DISH_OPEN), DISH_R * Math.sin(DISH_OPEN) * Math.sin(i * Math.PI * 2 / 3)));

// What only stands there: on a board a pin header, a heat sink on its chip and a coin
// cell beside a crystal; in the galaxy a landing pad, a dish listening to the sky and
// a ring of standing stones round a lit crystal.
const STANDING = {
  circuit: [
    { // a pin header, two rows of gold pins in a black shroud, and a smaller one beside it
      head: merge([
        outline(1.2, 0.36, '#e8ecef', 0, -0.2, 0.012), block(1.1, 0.07, 0.26, 0, 0, -0.2, '#1d1f22'),
        ...Array.from({ length: 16 }, (_, i) => block(0.022, 0.16, 0.022, -0.48 + (i % 8) * 0.137, 0, -0.26 + Math.floor(i / 8) * 0.12, '#d9b04c')),
        outline(0.5, 0.2, '#e8ecef', -0.2, 0.3, 0.012), block(0.44, 0.06, 0.14, -0.2, 0, 0.3, '#1d1f22'),
        ...Array.from({ length: 4 }, (_, i) => block(0.022, 0.14, 0.022, -0.36 + i * 0.107, 0, 0.3, '#d9b04c')),
      ]),
      size: [1.2, 0.8], scale: 3,
      posts: [[-0.3, -0.2, 0.15], [0.3, -0.2, 0.15], [-0.2, 0.3, 0.1]],
    },
    { // a heat sink on its chip
      head: merge([
        outline(0.86, 0.86, '#e8ecef', 0, 0, 0.012), block(0.62, 0.05, 0.62, 0, 0, 0, '#1d1f22'),
        block(0.7, 0.04, 0.7, 0, 0.05, 0, '#a9b1b9'),
        ...Array.from({ length: 8 }, (_, i) => block(0.7, 0.22, 0.018, 0, 0.09, -0.31 + i * 0.0886, '#bcc4cc')),
      ]),
      size: [0.86, 0.86], scale: 3.5,
      posts: [[0, 0, 0.36]],
    },
    { // a coin cell in its holder, and a crystal in its can
      head: merge([
        ring(0.3, '#e8ecef', -0.25, 0, 0, Math.PI * 2, 0.012),
        painted(new THREE.CylinderGeometry(0.27, 0.27, 0.05, 24).translate(-0.25, 0.025, 0), '#202326'),
        painted(new THREE.CylinderGeometry(0.23, 0.23, 0.035, 24).translate(-0.25, 0.0675, 0), '#c8ced4'), block(0.08, 0.01, 0.02, -0.25, 0.086, 0, '#7d848b'), block(0.02, 0.01, 0.08, -0.25, 0.086, 0, '#7d848b'),
        outline(0.36, 0.16, '#e8ecef', 0.38, 0, 0.012),
        painted(new THREE.CylinderGeometry(0.06, 0.06, 0.26, 12).scale(1, 1, 0.45).rotateZ(Math.PI / 2).translate(0.38, 0.065, 0), '#cfd5db'),
      ]),
      size: [1.15, 0.6], scale: 3,
      posts: [[-0.25, 0, 0.28], [0.38, 0, 0.1]],
    },
  ],
  galaxy: [
    { // a landing pad: a hexagon of plating rimmed with light, lamps at its corners
      head: merge([
        painted(new THREE.CylinderGeometry(0.75, 0.78, 0.035, 6).translate(0, 0.0175, 0), '#2a2740'),
        painted(new THREE.TorusGeometry(0.6, 0.012, 4, 6).rotateX(Math.PI / 2).rotateY(Math.PI / 6).translate(0, 0.04, 0), '#6ff4ff'),
        painted(new THREE.TorusGeometry(0.22, 0.012, 4, 24).rotateX(Math.PI / 2).translate(0, 0.04, 0), '#6ff4ff'),
        ...Array.from({ length: 6 }, (_, i) => {
          const a = i * Math.PI / 3;
          return merge([post(Math.cos(a) * 0.72, Math.sin(a) * 0.72, 0.1, 0.012, '#3a3552'),
            painted(new THREE.IcosahedronGeometry(0.025, 0).translate(Math.cos(a) * 0.72, 0.11, Math.sin(a) * 0.72), '#ffd27a')]);
        }),
      ]),
      size: [1.56, 1.56], scale: 3.5,
      posts: Array.from({ length: 6 }, (_, i) => [Math.cos(i * Math.PI / 3) * 0.72, Math.sin(i * Math.PI / 3) * 0.72, 0.03]),
      glow: { color: '#ffd27a', shells: lit(PAD_LAMPS, [[0.05, 0.5], [0.1, 0.18]]) },
    },
    { // a dish listening to the sky, hinged on its mast, its feed held over the bowl
      head: merge([
        post(0, 0, 0.05, 0.2, '#2f2b45'), post(0, 0, 0.33, 0.04, '#4a4466', 0.05),
        painted(new THREE.SphereGeometry(0.045, 8, 6).translate(...DISH_HINGE), '#4a4466'),
        painted(dishFrame(new THREE.SphereGeometry(DISH_R, 18, 6, 0, Math.PI * 2, Math.PI - DISH_OPEN, DISH_OPEN)), '#cfd2e6', 0.05),
        ...DISH_STRUTS.map(at => bar(at, DISH_FEED, 0.006, '#8b86a8')),
        painted(new THREE.IcosahedronGeometry(0.03, 0).translate(...DISH_FEED), '#6ff4ff'),
      ]),
      size: [0.8, 0.8], scale: 3,
      posts: [[0, 0, 0.2]],
      glow: { color: '#6ff4ff', shells: lit([DISH_FEED], [[0.06, 0.45], [0.12, 0.15]]) },
    },
    { // a ring of standing stones round a lit crystal
      head: merge([
        ring(0.55, '#6ff4ff', 0, 0, 0, Math.PI * 2, 0.02),
        ...Array.from({ length: 7 }, (_, i) => {
          const a = i * Math.PI * 2 / 7;
          return painted(new THREE.BoxGeometry(0.1, 0.32 + 0.08 * Math.sin(i * 2.3), 0.06).rotateY(-a)
            .translate(Math.cos(a) * 0.62, (0.32 + 0.08 * Math.sin(i * 2.3)) / 2, Math.sin(a) * 0.62), '#3b3656', 0.2);
        }),
        painted(new THREE.ConeGeometry(0.07, 0.36, 5).translate(0, 0.18, 0), '#9a7cff', 0.15),
      ]),
      size: [1.36, 1.36], scale: 3,
      posts: [...Array.from({ length: 7 }, (_, i) => [Math.cos(i * Math.PI * 2 / 7) * 0.62, Math.sin(i * Math.PI * 2 / 7) * 0.62, 0.06]), [0, 0, 0.07]],
      glow: { color: '#b49cff', shells: lit([[0, 0.22, 0]], [[0.12, 0.3], [0.24, 0.1], [0.4, 0.04]]) },
    },
  ],
};

// Nothing on one moves or is played; seen half as often as a game.
const still = a => ({ ...a, rigs: [], play: [], weight: 0.5 });

/** Each style's amenities. */
export const AMENITIES = {
  city: [...sports(CITY), ...playgrounds(CITY)],
  circuit: [...sports(CIRCUIT), ...playgrounds(CIRCUIT).slice(0, 2), ...STANDING.circuit.map(still)],
  galaxy: [...sports(GALAXY, LIGHT), ...playgrounds(GALAXY, LIGHT).slice(0, 2), ...STANDING.galaxy.map(still)],
};

const UP = new THREE.Vector3(0, 1, 0);
const _q = new THREE.Quaternion(), _p = new THREE.Vector3(), _s = new THREE.Vector3();

/** Where an amenity placed as `it` ({x, y, z, turn}) puts the point `local` of its model, into `out`. */
export function onAmenity(it, a, local, out = new THREE.Vector3()) {
  const k = a.scale ?? 1, c = Math.cos(it.turn), s = Math.sin(it.turn);
  return out.set(it.x + (local[0] * c + local[2] * s) * k, it.y + local[1] * k, it.z + (-local[0] * s + local[2] * c) * k);
}

/** The point of an amenity's model, [x, y, z], that the world point `world` is, for one placed as `it`. */
export function inAmenity(it, a, world) {
  const k = a.scale ?? 1, c = Math.cos(it.turn), s = Math.sin(it.turn), dx = world.x - it.x, dz = world.z - it.z;
  return [(dx * c - dz * s) / k, (world.y - it.y) / k, (dx * s + dz * c) / k];
}

/** The matrix that stands an amenity's model where `it` is: at its place, turned, at its scale. */
export function amenityMatrix(it, a, m = new THREE.Matrix4()) {
  return m.compose(_p.set(it.x, it.y, it.z), _q.setFromAxisAngle(UP, it.turn), _s.setScalar(a.scale ?? 1));
}

const _turn = new THREE.Matrix4(), _at = new THREE.Matrix4();
const AXES = { x: new THREE.Vector3(1, 0, 0), y: UP, z: new THREE.Vector3(0, 0, 1) };

/** The matrix for a rig of an amenity placed as `it`, turned `angle` about its axis at its pivot. */
export function rigMatrix(it, a, rig, angle = rig.rest ?? 0, m = new THREE.Matrix4()) {
  amenityMatrix(it, a, m);
  m.multiply(_at.makeTranslation(rig.at[0], rig.at[1], rig.at[2]));
  return m.multiply(_turn.makeRotationAxis(AXES[rig.axis], angle));
}
