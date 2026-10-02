// What the walker throws and what it pays out on a line: a shot from the hand to where
// it lands or what it hits, a cast to its mark, a hook that holds or glances off, the
// line that pulls the walker after it, and the puff where a shot strikes. Mixed into
// Walker (walk.js).

import * as THREE from '../vendor/three.module.min.js';
import { clamp } from '../core/numbers.js';
import { EYE, WATER, REACH, holds, faceOf } from './walkbase.js';
import { hits } from './tools.js';
import { along, fly, GRAVITY } from './ballistics.js';

// What a tool throws when it says nothing about how: something solid, thrown at a
// middling speed. Every tool that throws does say (tools.js); this is here so that
// adding one cannot make it fall through the floor of the world instead.
const DEFAULT_FLIGHT = { speed: 24, gravity: GRAVITY, cd: 0.004 };
// The air for a walker that has none (a test's): still.
const CALM = new THREE.Vector3();
// The length of one step of a shot's flight, in seconds: what it hits is looked for
// after every one, so a nail that crosses half a unit a step cannot pass through a
// beetle between two frames. The guide (trajectory.js) steps the same way.
export const FLIGHT_STEP = 1 / 120;
// How quickly a shot locked on to a bug turns on to it, as a share of the way for every
// unit it flies: by the distance flown rather than the time, so a nail that crosses the
// street in a tenth of a second turns on to a beetle as surely as a bubble that drifts.
const HOMING = 0.35;
// The fastest the walker's own motion is passed on to what they throw: past this it was
// not motion but a jump of position - a relayout, a teleport.
const MOST_CARRIED = 16;
// A line that has stuck: the longest it may pull before letting go (so a hook on
// something that moved cannot strand anyone), how far in from a roof's edge it sets
// the walker down, and how close a thing has to be before pulling to it is nothing.
// How fast and how near is the tool's business (tools.js).
const GRAPPLE_TIME = 5, ROOF_IN = 0.5, NO_PULL = 1.2;
// A hook that does not hold comes back off the wall: what of its speed it keeps, and
// how far out of the wall it is put so that it does not strike the same face again.
const GLANCE_KEEP = 0.35, GLANCE_OUT = 0.05;
// ... and how it goes from there: dropping under real gravity and tumbling end over
// end for GLANCE_DROP seconds, lying where it lands if it lands sooner, and then
// reeled back to the hand over GLANCE_REEL, skipping a little on the way.
const GLANCE_GRAVITY = 13, GLANCE_TUMBLE = 14, GLANCE_DROP = 0.7, GLANCE_REEL = 0.45;
// The puff where it struck: how long it lasts and how far it spreads.
const PUFF_TIME = 0.35, PUFF_GROW = 5;
// How far a dart looks for a wall to steer towards, and how nearly ahead of itself it
// will accept one, as a cosine: about forty degrees either side, which is wide enough
// to save a lobbed shot and narrow enough that a dart cannot turn round.
const TRACK_REACH = 50, TRACK_AHEAD = 0.75;
const FORWARD = new THREE.Vector3(0, 0, 1); // the dart geometry's nose

export const shots = {
  // Using the primary tool sends whatever it throws off along the view, carried on by
  // the walker's own motion, under its physics in the wind, until it hits something,
  // falls into the water or reaches the end of its reach - the path the guide drew
  // (trajectory.js), which is how it is aimed. A tool that throws nothing reaches
  // what the crosshair is on. A
  // tool that throws nothing reaches what it is pointed at the moment it is used - as
  // far as it reaches, which for the camera is any distance and for the net is arm's
  // length.
  // Implements: REQ-HUNT-037, REQ-HUNT-038, REQ-HUNT-044, REQ-TOOL-029
  fire() {
    if (this.still || this.dying !== null) return;
    // A photograph is up: the click that would have taken another one puts this one
    // away instead, which is the obvious thing to do with a picture held in front of
    // your face and saves waiting out the rest of the timer.
    if (this.showing) return this.endShow();
    // Empty-handed, a click near something to play with plays with it (play.js).
    if (this.playClick()) return;
    const tool = this.primary;
    this.firedAt = performance.now();
    this.swing = 0; // the hand moves whether or not anything flies
    const target = this.aimed(), bug = this.aim.bug;

    if (!tool.projectile) {
      // A camera keeps every frame it is used on, because pressing the shutter is what
      // a camera is for and anything less leaves its ordinary use doing nothing that
      // can be seen - every one but the use that asks to read a module it has already
      // tagged, which is the second click on a building and wants the details rather
      // than another picture of the same wall. What was in the frame goes with the
      // picture, so it is more than a number in a list: the bug that was under the
      // reticle, or the building behind it.
      // A bug in front of a tagged wall is still a bug: the rule is about the second
      // click on a building, not about everything standing in front of one.
      const again = !bug && !!target && this.tagged.has(target.node.id);
      const keeping = tool.keeps && !again;
      // The shutter goes off when a picture is taken and not when one is not.
      if (tool.flash && keeping) this.screenFlash();
      if (keeping) {
        this.hooks.onPhoto?.(bug ? `${bug.f.severity}: ${bug.f.title}` : target?.node.name || '');
      }
      if (bug) this.bugs.catch(bug, tool.catchAs);
      else if (target) this.tag(target);
      else if (this.aim.far) this.flash(`Out of reach: the ${tool.label.toLowerCase()} has to be walked up to`);
      return;
    }
    const shot = this.shotFrom(tool, this.viewmodel);
    this.loose(shot);
    // Locked on to a bug (trajectory.js): the shot homes on it as the guide showed it
    // would, and follows it if it walks on.
    shot.homing = this.prediction?.lock || null;
  },

  /**
   * The extinguisher, held on a burning building.
   *
   * Dousing is held rather than fired, which is the whole difference between this tool
   * and the rest of the bag. Every other primary tool is a gesture with a result: one
   * cast, one shot, one photograph. Fire does not answer to a gesture - it answers to
   * standing there and keeping the cone on it - so this is paid in seconds and not in
   * clicks, and the tool's own cadence is only what makes the foam keep coming.
   *
   * Putting out the building an advisory names puts out everything that fire lit,
   * wherever it has reached. That is not a mercy: it is what upgrading the dependency
   * actually does.
   */
  douse(deltaTime) {
    if (!this.fires || !this.primary.douses || !this.firing || this.still) return;
    const box = this.aim.box;
    if (!box) return;
    const what = this.fires.douse(box.node.id, deltaTime);
    if (what === 'out') this.flash(`Out - ${box.node.name} and everything that fire reached`);
    else if (what === 'cooled') this.flash(`${box.node.name} is out; the fire is still going elsewhere`);
  },

  /**
   * What a tool throws, leaving the muzzle of the hand that threw it, with its line
   * behind it if it trails one. Where it goes from there is the caller's business: the
   * crosshair's target for the hand the crosshair belongs to, and straight ahead for
   * the other one.
   *
   * Implements: REQ-TOOL-004, REQ-TOOL-038, REQ-TOOL-040
   */
  shotFrom(tool, vm) {
    const p = this.p;
    const start = this.muzzle(vm) || new THREE.Vector3(p.x, p.feet + EYE - 0.08, p.z);
    const mesh = tool.projectile(this.scene);
    mesh.position.copy(start);
    this.scene.scene.add(mesh);
    const shot = { mesh, t: 0, tool, hand: vm, start, flight: tool.flight || DEFAULT_FLIGHT };
    if (tool.line) { // a cast trails its line back to the hand it left
      const geo = new THREE.BufferGeometry().setFromPoints([start.clone(), start.clone()]);
      // Bendable like everything else out there: a line across a street on a small
      // planet is long enough for the curve to show.
      shot.line = new THREE.Line(geo, this.scene.bendable(new THREE.LineBasicMaterial({ color: tool.line })));
      shot.line.frustumCulled = false;
      this.scene.scene.add(shot.line);
    }
    this.darts.push(shot);
    return shot;
  },

  // Implements: REQ-TOOL-038
  /** Sends a shot off along the view, to fly on under its own physics. */
  loose(shot) {
    shot.vel = this.launch(shot.flight, new THREE.Vector3(), true);
    // Where it left from, so how far it has carried can be measured against the tool's
    // reach - and against the length of a line, for the ones that pay one out.
    shot.from = shot.start.clone();
  },

  /**
   * How fast and which way a shot leaves, into `out`: along the view at the tool's
   * speed, a nail knocked off line by its scatter (`scatter`, which the guide leaves
   * out), and carried on by however the walker is moving - running, falling, flying -
   * as anything thrown from a moving hand is.
   *
   * Implements: REQ-TOOL-081, REQ-TOOL-082
   */
  launch(flight, out, scatters = false) {
    const p = this.p;
    out.set(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.04, -Math.cos(p.yaw) * Math.cos(p.pitch));
    if (scatters && flight.spread) scatter(out, flight.spread);
    out.normalize().multiplyScalar(flight.speed);
    const moving = this.moving;
    if (moving && moving.lengthSq() < MOST_CARRIED * MOST_CARRIED) out.add(moving);
    return out;
  },

  /**
   * One step of a shot's flight, `h` seconds long, and what it is touching after it:
   * steered, if it steers, and flown under its physics in the wind. Changes nothing but
   * the shot, so the guide can fly a stand-in shot down the same path (trajectory.js).
   * Returns { bug, box }.
   *
   * Implements: REQ-TOOL-081, REQ-TOOL-082
   */
  flightStep(dart, h) {
    const position = dart.mesh.position, flight = dart.flight || DEFAULT_FLIGHT;
    // A shot locked on to a bug turns on to it, a little at a time, wherever it has
    // walked to (trajectory.js decides what is locked on to, and draws it).
    // Implements: REQ-TOOL-083
    if (dart.homing && !dart.homing.caught) this.turnTo(dart, dart.homing.position, HOMING * dart.vel.length() * h);
    // A tracking dart earns the name on a miss: its fins pull it round towards
    // whatever wall lies ahead of it, so a shot lobbed over a block still finds
    // one. Nothing else here steers, which is the whole of the difference between
    // it and a nail.
    else if (flight.track) this.steer(dart, flight.track * h);
    fly(position, dart.vel, flight, this.airNow || CALM, h);
    // Anything thrown catches a bug it passes through, if it is the kind of thing
    // that catches bugs at all.
    const bug = hits(dart.tool, 'bugs') ? this.bugs?.at(position) ?? null : null;
    return { bug, box: this.boxAt(position) };
  },

  /**
   * A line has stuck, and it pulls: the walker goes along it to what it caught.
   *
   * A line that bit a building sets them on top of it, which is how a facade is got
   * up and a roof arrived on - and the grapple, aimed off that roof at anything
   * lower, is how they get down again. Whether it bit at all is holds'.
   *
   * Returns whether the line was taken up. It may not be - too far to pull from, or
   * already there - and the caller has to know, because a hook that is not holding
   * anything has to come off the map with the rest of the shot.
   */
  hook(at, box, shot) {
    const line = shot.tool.reel;
    const to = at.clone();
    if (line.onto && box && box.kind !== 'land' && box.kind !== 'terrace') {
      // Onto it rather than against it: the top of the box, a step in from the face
      // so the landing is on the roof and not on its edge.
      to.y = box.y + box.h + 0.02;
      to.x += clamp(box.x - to.x, -ROOF_IN, ROOF_IN);
      to.z += clamp(box.z - to.z, -ROOF_IN, ROOF_IN);
    } else if (!box || box.kind === 'land' || box.kind === 'terrace') {
      to.y = this.height(to.x, to.z);
    }
    const away = Math.hypot(to.x - this.p.x, to.y - this.p.feet, to.z - this.p.z);
    // A hook that lands where the walker already is pulls them nowhere: looking
    // straight down from a roof catches that roof. Say so instead of paying out a
    // line and reeling in nothing - from up here, the way down is over the edge.
    if (away < Math.max(NO_PULL, line.stop)) {
      if (line.onto) this.flash('Nothing to be pulled to - aim past the edge');
      return false;
    }
    // And a line only so long: past that the cast still lands, it simply does not
    // drag the walker the width of the map to where it landed.
    if (away > line.max) {
      this.flash('Too far for the line');
      return false;
    }
    const up = to.y > this.p.feet;
    this.cutLine();
    // Whether Space is still down from a jump when the line bites (step).
    const jumping = this.keys.has('Space');
    this.pull = { to, t: 0, line, mesh: shot.mesh, rope: shot.line, hand: shot.hand, jumping };
    this.p.fly = this.flying(); // only one thing is carried, so a line is not a jet
    this.p.vy = 0;
    // A line is not a brake. Reeled down, the walker is falling as far as the ground
    // is concerned, and arriving at the bottom costs what dropping that far would -
    // counted from wherever the fall began, a jump before the shot included, and
    // carried on if the line is cut on the way. Reeled up, a fall in progress is over.
    // Implements: REQ-WALK-027
    this.fell = up ? null : Math.max(this.fell ?? this.p.feet, this.p.feet);
    this.flash(up ? 'Line away - going up' : 'Line away - going down');
    this.drawHud();
    return true;
  },

  /** Reels the walker along the line, and lets go at the end of it. */
  reel(deltaTime) {
    const p = this.p, to = this.pull.to;
    const dx = to.x - p.x, dy = to.y - p.feet, dz = to.z - p.z;
    const d = Math.hypot(dx, dy, dz);
    this.pull.t += deltaTime;
    if (d < this.pull.line.stop || this.pull.t > GRAPPLE_TIME) {
      this.cutLine();
      // Let go standing on what was arrived at, rather than falling back off it.
      p.feet = Math.max(p.feet, this.height(p.x, p.z));
      p.vy = 0;
      p.ground = true;
      return;
    }
    const step = Math.min(d, this.pull.line.speed * deltaTime);
    p.x += (dx / d) * step;
    p.feet += (dy / d) * step;
    p.z += (dz / d) * step;
    p.vy = 0;
    p.ground = false;
    // The hook stays where it bit, and the line follows the hand to it.
    if (this.pull.mesh) this.pull.mesh.position.copy(to);
    if (this.pull.rope) {
      const tip = this.muzzle(this.pull.hand) || new THREE.Vector3(p.x, p.feet + EYE - 0.05, p.z);
      this.pull.rope.geometry.setFromPoints([tip, to.clone()]);
    }
  },

  /**
   * A hook that struck `box` and did not hold: it comes back off the face it hit,
   * mirrored and slowed, and falls from there under its own physics until it lands -
   * so a miss looks like one, rather than a line that simply vanishes. Returns true,
   * for the caller that wants to know it went on.
   *
   * Implements: REQ-TOOL-067, REQ-TOOL-069
   */
  glance(shot, box) {
    const at = shot.mesh.position;
    const vel = shot.vel ? shot.vel.clone()
      : at.clone().sub(shot.start).normalize().multiplyScalar(shot.flight.speed);
    const n = faceOf(box, at);
    vel.addScaledVector(n, -2 * vel.dot(n)).multiplyScalar(GLANCE_KEEP);
    at.addScaledVector(n, GLANCE_OUT);
    Object.assign(shot, { vel, bug: null, target: null, to: null, from: null, glanced: true, since: 0, back: null });
    this.puff(at, n);
    this.flash(shot.tool.climbs
      ? 'The claw found nothing to close on - aim at the top of the wall'
      : 'The hook skipped off the wall - cast it onto the roof');
    return true;
  },

  /**
   * One frame of a hook that glanced off a wall (glance): it drops and tumbles, comes
   * to rest if it reaches the ground first, and then the line brings it back to the
   * hand - wherever the hand has got to since. Returns true once it is back.
   *
   * Implements: REQ-TOOL-067, REQ-TOOL-069
   */
  rebound(shot, deltaTime) {
    const m = shot.mesh;
    shot.since += deltaTime;
    if (shot.since < GLANCE_DROP) {
      if (shot.vel.lengthSq() > 0) {
        shot.vel.y -= GLANCE_GRAVITY * deltaTime;
        m.position.addScaledVector(shot.vel, deltaTime);
        m.rotation.x += GLANCE_TUMBLE * deltaTime;
        m.rotation.z += GLANCE_TUMBLE * 0.37 * deltaTime;
        const floor = Math.max(WATER, this.height(m.position.x, m.position.z));
        if (m.position.y <= floor) {
          m.position.y = floor;
          shot.vel.set(0, 0, 0); // lying where it fell until the line takes it up
        }
      }
      return false;
    }
    shot.back ??= m.position.clone();
    const u = Math.min(1, (shot.since - GLANCE_DROP) / GLANCE_REEL);
    const hand = this.muzzle(shot.hand) || new THREE.Vector3(this.p.x, this.p.feet + EYE - 0.05, this.p.z);
    m.position.lerpVectors(shot.back, hand, u * u);
    m.position.y += 0.25 * Math.sin(Math.PI * u) * (1 - u); // skipping as it is dragged in
    m.rotation.x += GLANCE_TUMBLE * 0.5 * deltaTime;
    return u >= 1;
  },

  /**
   * A small burst where a hook struck and did not hold, flat against the face it hit,
   * opening out and fading - the one moment of the miss that is easy to lose sight of.
   */
  puff(at, normal) {
    const mat = this.scene.bendable(new THREE.MeshBasicMaterial({
      color: '#eef1f5', transparent: true, opacity: 0.9, depthWrite: false, side: THREE.DoubleSide,
    }));
    const mesh = new THREE.Mesh(new THREE.RingGeometry(0.015, 0.05, 20), mat);
    mesh.position.copy(at).addScaledVector(normal, 0.01);
    mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), normal);
    this.scene.scene.add(mesh);
    this.puffs.push({ mesh, t: 0 });
  },

  updatePuffs(deltaTime) {
    for (const puff of this.puffs) {
      puff.t += deltaTime;
      puff.mesh.scale.setScalar(1 + puff.t * PUFF_GROW);
      puff.mesh.material.opacity = 0.9 * Math.max(0, 1 - puff.t / PUFF_TIME);
    }
    for (const puff of this.puffs.filter(p => p.t >= PUFF_TIME)) this.dropPuff(puff);
  },

  dropPuff(puff) {
    this.scene.scene.remove(puff.mesh);
    puff.mesh.geometry.dispose();
    puff.mesh.material.dispose();
    this.puffs.splice(this.puffs.indexOf(puff), 1);
  },

  /** Takes everything thrown off the map, lines and puffs included. */
  dropDarts() {
    for (const dart of this.darts) {
      discard(dart.mesh);
      discard(dart.line);
    }
    this.darts = [];
    for (const puff of [...this.puffs]) this.dropPuff(puff);
  },

  /** Lets go of whatever the line is holding, and takes the line off the map. */
  cutLine(say) {
    if (!this.pull) return;
    discard(this.pull.mesh);
    discard(this.pull.rope);
    this.pull = null;
    if (say) this.flash(say);
  },

  // Implements: REQ-HUNT-001, REQ-HUNT-017, REQ-HUNT-018, REQ-TOOL-004, REQ-TOOL-027
  updateDarts(deltaTime) {
    const done = [], previous = new THREE.Vector3(), moved = new THREE.Vector3();
    for (const dart of this.darts) {
      dart.t += deltaTime;
      previous.copy(dart.mesh.position);
      // Off the wall and on its way back (rebound), cast at a mark, or a miss.
      const over = dart.glanced ? this.rebound(dart, deltaTime)
        : dart.bug || dart.target ? this.flyCast(dart)
          : this.flyFree(dart, deltaTime);
      if (over) done.push(dart);
      this.dressDart(dart, moved.subVectors(dart.mesh.position, previous), deltaTime);
    }
    for (const dart of done) {
      // A line that bit keeps its hook and its rope: they are what the walker is
      // being pulled along, and cutLine is what takes them off the map.
      if (!dart.kept) {
        discard(dart.mesh);
        discard(dart.line);
      }
      this.darts.splice(this.darts.indexOf(dart), 1);
    }
  },

  /** Flies a shot along the path it was aimed on, and lands it on its mark; whether it is over. */
  flyCast(dart) {
    const m = dart.mesh;
    dart.path ||= [dart.start.clone(), dart.to.clone()];
    dart.mark ||= dart.to.clone();
    const u = Math.min(1, dart.t / dart.T);
    along(dart.path, dart.T, dart.t, m.position);
    // A bug walks on while the shot is in the air, so the shot follows it: more of
    // the way it has walked the nearer the shot is to landing.
    if (dart.bug && !dart.bug.caught) {
      m.position.addScaledVector((this.following ||= new THREE.Vector3()).subVectors(dart.bug.position, dart.mark), u);
    }
    if (u < 1) return false;
    dart.vel ||= dart.end?.clone(); // how it struck, for a hook that glances off
    // A rod both lands the cast and hauls on it; a grapple only hauls.
    if (!dart.tool.climbs) {
      if (dart.bug) this.bugs.catch(dart.bug, dart.tool.catchAs);
      else if (dart.target) this.tag(dart.target);
    }
    // A line hauls on a wall, not on a beetle: what the rod caught comes back on the
    // line, and the walker stays where they are. A hook that does not hold flies on
    // off the wall instead, so it is not over yet.
    if (dart.tool.reel && !dart.bug) {
      if (!holds(dart.tool, dart.target, m.position)) {
        this.glance(dart, dart.target);
        return false;
      }
      if (this.hook(m.position, dart.target, dart)) dart.kept = true;
    }
    return true;
  },

  /**
   * Flies a miss on under the tool's own physics - a dart drops like a dart, a bubble
   * slows to a crawl and then climbs - and ends it on what it hits; whether it is over.
   */
  flyFree(dart, deltaTime) {
    const n = Math.max(1, Math.ceil(deltaTime / FLIGHT_STEP - 1e-9));
    for (let k = 0; k < n; k++) {
      const outcome = this.landed(dart, this.flightStep(dart, deltaTime / n));
      if (outcome) return outcome === 'over';
    }
    return false;
  },

  /**
   * What a step of flight came down on (flightStep): whatever it catches, tags, bites or
   * glances off. Returns 'over' when the shot is done, 'glanced' when it has come back
   * off a wall (rebound flies it from there), and null to fly on.
   */
  landed(dart, { bug, box: hit }) {
    const m = dart.mesh;
    if (bug) this.bugs.catch(bug, dart.tool.catchAs);
    // One that has already glanced off a wall is on its way down, spent: it tags
    // nothing and bites nothing on the way.
    if (hit && !dart.glanced && !dart.tool.climbs && hits(dart.tool, 'buildings')
      && hit.kind !== 'land' && hit.kind !== 'terrace') this.tag(hit);
    // A grapple bites anything solid, the ground included: that is what makes a
    // shot off a roof a way down rather than a wasted line. A rod does not - a
    // cast that falls short lands on the pavement, and a line that hauls the
    // walker a step across their own street is not worth having.
    const ground = hit && (hit.kind === 'land' || hit.kind === 'terrace');
    if (hit && dart.tool.reel && !dart.glanced && (dart.tool.climbs || !ground)) {
      if (!holds(dart.tool, hit, m.position)) {
        this.glance(dart, hit);
        return 'glanced';
      }
      if (this.hook(m.position, hit, dart)) dart.kept = true;
    }
    // A shot that hits nothing still has a range: what a tool reaches is what it
    // throws that far, and a nail that sails on over the next six blocks would be a
    // shot nobody aimed. A line is shorter still - fired into the sky or out over the
    // water a hook finds nothing to stop it, and without this it would be six seconds
    // of a rope across the view, going nowhere.
    const gone = dart.from ? m.position.distanceTo(dart.from) : 0;
    // A tool that pays out a line ends where the line does, and says so; for
    // everything else the end is the tool's reach.
    const rope = dart.tool.reel && dart.from && gone > dart.tool.reel.max;
    const spent = !dart.tool.reel && dart.from && gone > (dart.tool.reach ?? REACH);
    if (rope) this.flash('The line ran out');
    return bug || hit || rope || spent || m.position.y < WATER || dart.t > 6 ? 'over' : null;
  },

  /**
   * Turns and shapes a shot for the frame, having `moved` since the last: a dart
   * points along its flight; a hoop spins, a bubble wobbles, a bobber just bobs along.
   */
  dressDart(dart, moved, deltaTime) {
    const m = dart.mesh;
    if (m.userData.aim && !dart.glanced && moved.lengthSq() > 1e-10) {
      m.quaternion.setFromUnitVectors(FORWARD, moved.normalize());
    }
    if (m.userData.spin) m.rotation.z += m.userData.spin * deltaTime;
    if (m.userData.wobble) m.scale.set(1 + Math.sin(dart.t * 9) * 0.07, 1 - Math.sin(dart.t * 9) * 0.07, 1);
    // A cloud opens out as it goes: what left the horn as a gout is a fog by the
    // time it is across the street, which is why the extinguisher is forgiving up
    // close and no use at all past that.
    if (m.userData.swell) m.scale.setScalar(1 + dart.t * m.userData.swell);
    if (dart.line) { // keep the line between the hand it left and what was cast
      const tip = this.muzzle(dart.hand) || new THREE.Vector3(this.p.x, this.p.feet + EYE - 0.05, this.p.z);
      dart.line.geometry.setFromPoints([tip, m.position.clone()]);
    }
  },

  /**
   * Turns a shot in flight towards the wall it has picked, by at most `by` radians.
   *
   * It picks one on the way out of the muzzle and holds it: a dart that chose again
   * every frame would swing from building to building as it passed them, and it would
   * cost a sweep of the layout a frame to do it. Nothing is picked twice, and a shot
   * that leaves with nothing ahead of it stays a shot that misses.
   *
   * Implements: REQ-TOOL-027, REQ-TOOL-041
   */
  steer(dart, by) {
    const at = dart.mesh.position;
    const going = (this.aimAt ||= new THREE.Vector3()).copy(dart.vel).normalize();
    if (dart.lock === undefined) dart.lock = this.wallAhead(at, going);
    if (!dart.lock) return;
    const b = dart.lock;
    this.turnTo(dart, (this.aimTo ||= new THREE.Vector3()).set(b.x, b.y + b.h / 2, b.z), by);
  },

  /** Turns a shot in flight towards `to` by at most `by` of the way, keeping its speed. */
  turnTo(dart, to, by) {
    const at = dart.mesh.position;
    const going = (this.aimAt ||= new THREE.Vector3()).copy(dart.vel).normalize();
    const want = (this.aimWant ||= new THREE.Vector3()).subVectors(to, at);
    if (want.lengthSq() < 1e-6) return;
    const speed = dart.vel.length();
    dart.vel.copy(going.lerp(want.normalize(), Math.min(1, by)).normalize()).multiplyScalar(speed);
  },

  /**
   * The nearest box a dart could tag that lies within TRACK_AHEAD of where it is
   * going and TRACK_REACH of where it is. The ground and the blocks are not it: a dart
   * that steered into the street would never reach anything.
   */
  wallAhead(at, going) {
    let best = null, nearest = TRACK_REACH;
    for (const b of this.boxes) {
      if (b.kind === 'land' || b.kind === 'terrace') continue;
      const dx = b.x - at.x, dy = b.y + b.h / 2 - at.y, dz = b.z - at.z;
      const d = Math.hypot(dx, dy, dz);
      if (d >= nearest || d < 0.2) continue;
      if ((dx * going.x + dy * going.y + dz * going.z) / d < TRACK_AHEAD) continue;
      nearest = d;
      best = b;
    }
    return best;
  },
};

/**
 * Takes what a shot threw - its mesh, or the line it trailed - off the map, and frees
 * the geometries and materials it was drawn with: taken off and nothing more, they
 * stay on the GPU for as long as the page is open, and a nail gun held down makes
 * dozens a second. Every shot is built afresh (tools.js projectile, shotFrom), so
 * nothing in one is shared with anything still being drawn. Nothing is nothing to do.
 */
function discard(object) {
  if (!object) return;
  object.removeFromParent();
  object.traverse(part => {
    part.geometry?.dispose();
    for (const material of [part.material].flat()) material?.dispose();
  });
}

// Implements: REQ-TOOL-027, REQ-TOOL-042
/** Knocks a point or a direction off course by up to `by`, evenly in all directions. */
export function scatter(v, by) {
  v.x += (Math.random() * 2 - 1) * by;
  v.y += (Math.random() * 2 - 1) * by;
  v.z += (Math.random() * 2 - 1) * by;
}
