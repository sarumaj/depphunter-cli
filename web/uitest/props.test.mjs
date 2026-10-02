// The plants the city stands among, when the drawn models are not there.
//
// props.glb is fetched after the map is up, and a page without it - a test, a slow
// network, a file that failed to load - dresses the map from the tables in city.js
// instead. Those tables are the fallback nobody sees while the models work, which is
// how a tree in them could go on blocking the walker with nothing drawn to show for it.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const THREE = await import('../static/vendor/three.module.min.js');
const { makeProps, setNight, rampsFor, rampHeight, amenitySpan, amenitiesOf } = await import('../static/map/city.js');
const { raised, buildParameters } = await import('../static/map/buildings.js');

describe('vegetation without the plant models', () => {
  // Verifies: REQ-CITY-022
  it('draws every tree it makes an obstacle of', () => {
    const land = box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 });
    const group = makeProps([land], m => m, 'city');
    assert.ok(group.userData.obstacles.length, 'a shore with no trees on it');
    // Two kinds a species, trunk then crown, before the bushes and the lamps; each
    // kind drawn in full and coarse (lod.js).
    const species = group.children.filter(mesh => !mesh.userData.coarse).slice(0, 6);
    for (const mesh of species) {
      const position = mesh.geometry.getAttribute('position');
      assert.ok(position && position.count > 0, 'a tree species drawn with no geometry');
      assert.ok(mesh.count > 0, 'a tree species with no instances drawn');
    }
  });
});

describe('the parts on a circuit board', () => {
  // Verifies: REQ-MAP-055
  it('solders every kind of part somewhere on a board', () => {
    const land = box('land', 0, 0, 40, 40, { y: -0.45, h: 0.45 });
    const group = makeProps([land], m => m, 'circuit');
    // A stem and a head for each of the eight through-hole kinds, then the four
    // surface-mount ones, each drawn in full and coarse (lod.js).
    const drawn = group.children.filter(mesh => !mesh.userData.coarse && !mesh.userData.glow).slice(0, 8 * 2 + 4);
    assert.equal(drawn.length, 20);
    for (const mesh of drawn) assert.ok(mesh.count > 0, 'a kind of part stands nowhere');
  });
});

describe('what stands in the galaxy', () => {
  // Verifies: REQ-MAP-057
  it('grows every kind of thing somewhere, and lights some of them day and night', () => {
    const land = box('land', 0, 0, 40, 40, { y: -0.45, h: 0.45 });
    const block = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.2 });
    const group = makeProps([land, block], m => m, 'galaxy');
    // A stem and a head for each of the seven kinds, with a lit one's shells after
    // its head, then the three low ones (lod.js draws each kind in full and coarse).
    const drawn = group.children.filter(mesh => !mesh.userData.coarse && !mesh.userData.glow).slice(0, 7 * 2 + 3);
    for (const mesh of drawn) assert.ok(mesh.count > 0, 'a kind of prop stands nowhere');
    const glow = group.children.filter(mesh => mesh.userData.glow && !mesh.userData.coarse && mesh.userData.amenity === undefined);
    assert.equal(glow.length, 2 + 3 + 3, 'the fungi, the pods or the beacons do not glow');
    assert.ok(glow.every(mesh => mesh.visible && mesh.count > 0), 'a glow is out by day');
    const colors = glow.map(mesh => mesh.material.color.getHex());
    setNight(group, true);
    assert.ok(glow.every(mesh => mesh.visible), 'a glow is out at night');
    assert.deepEqual(glow.map(mesh => mesh.material.color.getHex()), colors, 'a glow dims after dark');
    assert.ok(glow.every(mesh => mesh.material.opacity === mesh.userData.glow * 1.5), 'a glow is no stronger at night');
  });
});

describe('what a park holds', () => {
  // Where each kind of amenity stands in a dressed map, and which way round.
  const placed = (group, style) => amenitiesOf(style).map((_, kind) => group.userData.amenities.filter(it => it.kind === kind));
  // The ground one covers, where it stands.
  const ground = (style, kind, at) => {
    const { w, d } = amenitySpan(amenitiesOf(style)[kind], at.turn);
    return { x0: at.x - w / 2, x1: at.x + w / 2, z0: at.z - d / 2, z1: at.z + d / 2 };
  };

  // Verifies: REQ-CITY-039
  it('makes them the size they are, beside the walker', () => {
    // A unit is about three and a half meters: the walker's eye is at 0.45.
    const [pitch, court, volley, ...playgrounds] = amenitiesOf('city').map(a => amenitySpan(a));
    assert.ok(pitch.w > 7 && pitch.w < 12 && pitch.d > 4.5, `a pitch ${pitch.w} by ${pitch.d}`);
    assert.ok(court.w > 4.5 && court.w < 9, `a court ${court.w} long`);
    assert.ok(volley.w > 5 && volley.w < 8 && volley.d > 3, `a volleyball court ${volley.w} by ${volley.d}`);
    for (const playground of playgrounds) assert.ok(playground.w > 2.5 && playground.w < 5, `a playground ${playground.w} across`);
  });

  // Verifies: REQ-CITY-039
  it('gives some of the lawn to every kind of amenity, in every style, and leaves the rest lawn', () => {
    for (const style of ['city', 'circuit', 'galaxy']) {
      const park = box('terrace', 0, 0, 120, 120, { y: 0, h: 0.28 });
      const kinds = placed(makeProps([park], m => m, style), style);
      for (const at of kinds) assert.ok(at.length > 0, `a kind of ${style} amenity stands nowhere`);
      const covered = kinds.flatMap((at, kind) => at.map(it => ground(style, kind, it)))
        .reduce((sum, r) => sum + (r.x1 - r.x0) * (r.z1 - r.z0), 0);
      assert.ok(covered < 0.5 * 120 * 120, `${style}'s amenities cover ${covered} of the park's ${120 * 120}`);
      // ... and never one on another.
      const all = kinds.flatMap((at, kind) => at.map(it => ground(style, kind, it)));
      for (const [i, r] of all.entries()) {
        for (const o of all.slice(i + 1)) assert.ok(r.x1 <= o.x0 || o.x1 <= r.x0 || r.z1 <= o.z0 || o.z1 <= r.z0, 'two amenities overlap');
      }
    }
  });

  // Verifies: REQ-CITY-039
  it('keeps them on lawn, plants nothing on them, and puts their posts in the walker\'s way', () => {
    const park = box('terrace', 0, 0, 60, 60, { y: 0, h: 0.28 });
    const tower = box('building', 0, 0, 6, 6, { y: 0.28, h: 3 });
    tower.node.parentNode = park.node;
    const group = makeProps([park, tower], m => m, 'city');
    // A goal's net is a sheet: drawn from one face only, it vanishes from behind the goal.
    const drawn = group.children.filter(mesh => mesh.userData.amenity !== undefined && !mesh.userData.glow);
    assert.ok(drawn.length && drawn.every(mesh => mesh.material.side === THREE.DoubleSide), 'an amenity is drawn from one side only');
    const kinds = placed(group, 'city');
    assert.ok(kinds.flat().length, 'no amenity in a park this size');
    const { obstacles } = group.userData;
    // The goalposts, the hoops' poles, the playground's legs and posts - and nothing
    // else: a tree planted on one would be one more.
    const POSTS = amenitiesOf('city').map(a => a.posts.length);
    kinds.forEach((at, kind) => {
      for (const it of at) {
        const r = ground('city', kind, it);
        assert.ok(Math.max(r.x0 - 3, -3 - r.x1, r.z0 - 3, -3 - r.z1) >= 0.94, `amenity ${kind} at ${it.x}, ${it.z} is on the street round the tower`);
        assert.ok(r.x0 >= -30 + 0.94 && r.x1 <= 30 - 0.94 && r.z0 >= -30 + 0.94 && r.z1 <= 30 - 0.94, 'an amenity is on the ring road');
        const inside = obstacles.filter(o => o.x > r.x0 && o.x < r.x1 && o.z > r.z0 && o.z < r.z1);
        assert.equal(inside.length, POSTS[kind], `amenity ${kind} at ${it.x}, ${it.z} has ${inside.length} things in the way`);
      }
    });
  });
});

describe('playgrounds', () => {
  const rides = a => new Set(a.play.filter(e => e.ride).map(e => e.ride));

  // Verifies: REQ-CITY-040
  it('come in three kinds in a city and two on a board and in the galaxy, beside the same three games', () => {
    const city = amenitiesOf('city').filter(a => rides(a).size);
    assert.equal(city.length, 3);
    assert.equal(new Set(city.map(a => a.play.filter(e => e.ride).map(e => e.ride).sort().join())).size, 3, 'two of the city\'s playgrounds are the same');
    for (const style of ['city', 'circuit', 'galaxy']) {
      const all = amenitiesOf(style);
      assert.ok(all.filter(a => rides(a).size).length >= 2, `${style} has fewer than two playgrounds`);
      const balls = all.flatMap(a => a.play.filter(e => e.ball).map(e => e.ball)).sort();
      assert.deepEqual(balls, ['basket', 'soccer', 'volley'], `${style} plays ${balls}`);
      const at = all.flatMap(a => a.play);
      assert.equal(at.filter(e => e.goal).length, 2);
      assert.equal(at.filter(e => e.hoop).length, 2);
      assert.equal(at.filter(e => e.net).length, 1);
      // Every ride is on something that moves.
      for (const a of all) for (const e of a.play) if (e.rig !== undefined) assert.ok(a.rigs[e.rig], `a ${e.ride} has no rig`);
    }
  });

  // Verifies: REQ-CITY-040
  it('draws what moves apart, at rest, and moves only the one posed', () => {
    const park = box('terrace', 0, 0, 120, 120, { y: 0, h: 0.28 });
    const group = makeProps([park], m => m, 'city');
    const swung = group.userData.amenities.find(it => it.rigs?.length);
    assert.ok(swung, 'nothing in a park this size moves');
    const { scatter, item } = swung.rigs[0];
    const before = scatter.matrices.slice();
    const m = new THREE.Matrix4().makeTranslation(1, 2, 3);
    scatter.pose(item, m);
    scatter.update();
    const changed = [...Array(before.length / 16).keys()].filter(i => before.slice(i * 16, i * 16 + 16).some((v, k) => v !== scatter.matrices[i * 16 + k]));
    assert.deepEqual(changed, [item.slot], 'posing one rig moved others, or not it');
  });
});

describe('street lamps at night', () => {
  // Verifies: REQ-CITY-036
  it('lights the city\'s lamps after dark only, with a glow and a pool on the pavement', () => {
    const land = box('land', 0, 0, 12, 12, { y: -0.45, h: 0.45 });
    const block = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.2 });
    const group = makeProps([land, block], m => m, 'city');
    const night = group.children.filter(mesh => mesh.userData.afterDark);
    assert.ok(night.length, 'the city\'s lamps have nothing to light');
    assert.ok(night.some(mesh => mesh.count > 0), 'no lamp is lit');
    assert.ok(night.every(mesh => !mesh.visible), 'a lamp is lit before dark');
    setNight(group, true);
    assert.ok(night.every(mesh => mesh.visible), 'a lamp stayed dark at night');
    setNight(group, false);
    assert.ok(night.every(mesh => !mesh.visible), 'a lamp stayed lit by day');
  });

  // Verifies: REQ-CITY-036
  it('leaves an LED lit whether or not it is dark', () => {
    const block = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.2 });
    const group = makeProps([block], m => m, 'circuit');
    const glow = group.children.filter(mesh => mesh.userData.glow);
    assert.ok(glow.length && glow.every(mesh => mesh.visible && !mesh.userData.afterDark));
  });
});

describe('the way up from one level to the next', () => {
  // The street's level, a block on it, a level on that block and one on that level:
  // a terrace's node is its kids' parentNode, which is how blocks() finds what stands
  // on what.
  const street = box('terrace', 0, 0, 16, 16, { y: 0, h: 0.28 });
  const ground = box('terrace', 0, 0, 11, 11, { y: 0.28, h: 0.28 });
  const middle = box('terrace', 0, 0, 7, 7, { y: 0.56, h: 0.28 });
  const top = box('terrace', 0, 0, 3, 3, { y: 0.84, h: 0.28 });
  ground.node.parentNode = street.node;
  middle.node.parentNode = ground.node;
  top.node.parentNode = middle.node;

  // Verifies: REQ-CITY-037
  it('lays the ground out in roads and paves the levels stacked on it as plazas', () => {
    assert.equal(raised(street), false);
    assert.equal(raised(ground), false);
    assert.equal(raised(middle), true);
    assert.equal(raised(top), true);
    assert.equal(buildParameters(ground)[2], 0);
    assert.equal(buildParameters(middle)[2], 1, 'the shader is not told the level is a plaza');
  });

  // Verifies: REQ-CITY-037
  it('drives up to a road, and climbs stairs to a plaza', () => {
    const ramps = rampsFor([street, ground, middle, top]);
    const to = level => ramps.find(r => r.y1 === level.y + level.h);
    const up = to(ground), onto = to(middle), stairs = to(top);
    assert.ok(up && !up.stairs && up.drive, 'no ramp with a driveway up to the block on the street');
    assert.ok(onto?.stairs && !onto.drive, 'no stairs from the road up to the plaza');
    assert.ok(stairs?.stairs && !stairs.drive, 'no stairs between the plazas');
    assert.ok(stairs.len < up.len, 'the stairs are no shorter than a ramp');
    // The walker goes up a flight as up a ramp: from the lower level to the upper.
    const foot = [stairs.origin[0] + stairs.n[0] * stairs.width / 2, stairs.origin[1] + stairs.n[1] * stairs.width / 2];
    const head = [foot[0] + stairs.u[0] * (stairs.len - 0.01), foot[1] + stairs.u[1] * (stairs.len - 0.01)];
    assert.ok(Math.abs(rampHeight(stairs, ...foot) - stairs.y0) < 1e-9);
    assert.ok(Math.abs(rampHeight(stairs, ...head) - stairs.y1) < 1e-9);
  });

  // Verifies: REQ-CITY-037
  it('takes stairs up to a block too small for a ramp', () => {
    const land = box('terrace', 0, 0, 8, 8, { y: 0, h: 0.28 });
    const small = box('terrace', 0, 0, 1.4, 1.4, { y: 0.28, h: 0.28 });
    small.node.parentNode = land.node;
    const [way] = rampsFor([land, small]);
    assert.ok(way?.stairs && !way.drive, 'no way up to a block a ramp does not fit');
  });
});
