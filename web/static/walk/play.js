// Playing on what the parks hold (map/amenities.js), with empty hands: riding a swing,
// a roundabout, a seesaw, a spring rider or a slide, and playing ball - a shot at a
// hoop, a kick at a goal, a serve over a net. Mixed into Walker (walk.js).
//
// Only with nothing in either hand: tools put away (H) and nothing carried in the
// left. A walker holding a tool is told how to free their hands instead.
//
// A ride moves the walker and the one rig being ridden (map/lod.js pose); everything
// else in the park stays as it is drawn. A ball is the one thing out there with
// physics of its own: it falls, bounces and rolls once played - walked into, it stays
// put - and it knows the hoops, goals and net of its own court.
//
// Implements: REQ-WALK-057, REQ-WALK-058

import * as THREE from '../vendor/three.module.min.js';
import { onAmenity, inAmenity, rigMatrix } from '../map/amenities.js';
import { shaded, merge } from '../map/shapes.js';
import { EYE, WATER, POINT } from './walkbase.js';
import { ballInHands, viewLights, HELD_BALL } from './tools.js';
import { makeGuide } from './trajectory.js';

const RIDE_REACH = 0.5;     // how near a seat, a deck or a ladder has to be to get on
const SIT = 0.22;           // the eye over a seat
const REAL_G = 2.76;        // gravity in units, a unit being 3.55 m: what a swing swings by
const BALL_G = 4;           // ... and a ball falls by, a little brisker than the truth
const PICK = 0.45;          // how near a ball has to be to pick up, or to kick
const LOOKED_AT = 0.85;     // how square on (facing) a ball has to be looked at to be played
const BODY = 0.12;          // the walker's radius, as walk.js has it
const NEAR = 14, FAR = 26;  // a court's ball is put out within NEAR, and taken in past FAR
const STEP = 1 / 120;       // a ball's physics step
const NET_DEPTH = 0.08;     // how far back of its line a goal's net holds a ball, as the model has it
const BLEND = 0.3;          // seconds getting on to a ride takes
const SETTLE = 8;           // how fast the eye settles where it is going, getting on or off a seat
const KICK_LAG = 0.2;       // seconds from the click to the foot meeting the ball
const LOST = 1.2;           // seconds a ball lies off its court before it is put back
const FLIGHT = 3;           // seconds of a ball's flight the guide looks ahead at most
const UP = new THREE.Vector3(0, 1, 0);

const BALLS = {
  soccer: { r: 0.045, bounce: 0.55, roll: 0.7 },
  basket: { r: 0.048, bounce: 0.72, roll: 1.4 },
  volley: { r: 0.045, bounce: 0.6, roll: 1.6 },
};

const SAYS = {
  swing: 'ride the swing', spin: 'ride the roundabout', rock: 'ride it', slide: 'go down the slide',
  soccer: 'kick the ball', basket: 'pick up the ball', volley: 'pick up the ball',
};

const v1 = new THREE.Vector3(), v2 = new THREE.Vector3(), v3 = new THREE.Vector3(), v4 = new THREE.Vector3();
const m1 = new THREE.Matrix4();

export const play = {
  /** Whether both hands are free to play with: the tools put away and nothing carried. */
  freeHands() { return this.handsOff && !this.secondary; },

  /** The parks' amenities near the walker, within `reach` of their middle plus their size. */
  amenitiesNear(reach) {
    const all = this.scene.props?.userData.amenities || [];
    const p = this.p;
    return all.filter(it => {
      const k = it.spec.scale ?? 1, size = Math.max(...it.spec.size) * k / 2;
      return Math.hypot(it.x - p.x, it.z - p.z) < size + reach;
    });
  },

  /**
   * What a click would play now, if anything: the ball held, a ball at hand, a ride
   * within reach - nearest first. { kind: 'held' | 'ball' | 'ride', ... } or null.
   */
  playable() {
    if (this.ballHeld) return { kind: 'held', ball: this.ballHeld };
    const p = this.p, eye = v1.set(p.x, p.feet + EYE, p.z);
    let best = null, nearest = Infinity;
    for (const ball of this.balls.values()) {
      const d = Math.hypot(ball.pos.x - p.x, ball.pos.z - p.z);
      const low = ball.pos.y - p.feet < EYE + 0.2, inAir = ball.pos.distanceTo(eye) < 0.55 && ball.kind === 'volley';
      if ((d < PICK && low || inAir) && this.facing(ball.pos) > LOOKED_AT) {
        if (d < nearest) { nearest = d; best = { kind: 'ball', ball }; }
      }
    }
    if (best) return best;
    for (const it of this.amenitiesNear(RIDE_REACH)) {
      for (const entry of it.spec.play) {
        if (!entry.ride) continue;
        const d = this.rideDistance(it, entry);
        if (d < RIDE_REACH && d < nearest) { nearest = d; best = { kind: 'ride', it, entry }; }
      }
    }
    return best;
  },

  /** How square on the walker is facing a point: 1 straight at it, 0 across, -1 away. */
  facing(at) {
    const dx = at.x - this.p.x, dz = at.z - this.p.z, d = Math.hypot(dx, dz) || 1;
    return (-Math.sin(this.p.yaw) * dx - Math.cos(this.p.yaw) * dz) / d;
  },

  // How far the walker is from getting on a ride.
  rideDistance(it, entry) {
    const a = it.spec, p = this.p;
    if (entry.ride === 'spin') {
      const c = onAmenity(it, a, entry.pivot, v2);
      return Math.abs(Math.hypot(p.x - c.x, p.z - c.z) - entry.r * a.scale);
    }
    const at = entry.ride === 'slide' ? entry.path[0]
      : entry.ride === 'swing' ? [entry.pivot[0], entry.pivot[1] - entry.length, entry.pivot[2]]
        : [entry.pivot[0] + entry.seat[0], entry.pivot[1] + entry.seat[1], entry.pivot[2] + entry.seat[2]];
    const w = onAmenity(it, a, at, v2);
    return Math.abs(w.y - p.feet) > 0.6 ? Infinity : Math.hypot(p.x - w.x, p.z - w.z);
  },

  /** A click, with empty hands: whatever playable() found is played, and the click taken. */
  playClick() {
    if (!this.freeHands()) return false;
    if (this.riding) return true;
    const what = this.playable();
    if (!what) return false;
    if (what.kind === 'held') this.throwBall(what.ball);
    else if (what.kind === 'ball') this.touchBall(what.ball);
    else this.board(what.it, what.entry);
    return true;
  },

  /** Every frame: the balls, the rides settling back, and the line saying what a click would do. */
  updatePlay(deltaTime) {
    if (this.playProps && this.scene.props !== this.playProps) this.endPlay();
    this.playProps = this.scene.props;
    if (this.still) return;
    if (!this.freeHands()) {
      if (this.riding) this.leaveRide(false);
      if (this.ballHeld) this.dropBall();
    }
    this.putOutBalls();
    for (const ball of this.balls.values()) if (ball !== this.ballHeld && ball !== this.kicking?.ball) this.rollBall(ball, deltaTime);
    if (this.kicking && (this.kicking.left -= deltaTime) <= 0) {
      this.send(this.kicking.ball, 'kick');
      this.kicking = null;
    }
    this.settle(deltaTime);
    // Off a seat, the eye rises to standing, and the legs straighten, over a moment.
    const settling = Math.exp(-SETTLE * deltaTime);
    this.eyeShift = Math.abs(this.eyeShift) < 1e-3 ? 0 : this.eyeShift * settling;
    this.unseat = this.unseat < 1e-2 ? 0 : (this.unseat || 0) * settling;
    this.drawBallGuide();
    this.drawPlay();
  },

  /** Says what a click would play, or how to get both hands free for it. */
  drawPlay() {
    const el = this.playEl ||= this.hud?.querySelector('.w-play');
    if (!el) return;
    let text = '';
    if (this.riding) text = this.riding.entry.ride === 'slide' ? 'Space: jump off' : 'W: push · Space: jump off';
    else {
      const what = this.playable();
      if (what && !this.freeHands()) text = this.secondary ? 'Put down what is in your left hand to play' : 'H puts your tools away to play';
      else if (what?.kind === 'held') text = `Click: ${what.ball.kind === 'basket' ? 'shoot' : what.ball.kind === 'volley' ? (this.p.ground ? 'serve - jump first for a jump serve' : 'jump serve') : 'throw'}`;
      else if (what?.kind === 'ball') text = `Click: ${what.ball.kind === 'volley' && !what.ball.rest ? 'hit the ball' : SAYS[what.ball.kind]}`;
      else if (what) text = `Click: ${SAYS[what.entry.ride]}`;
    }
    if (el.textContent !== text) el.textContent = text;
  },

  /** Puts everything away: off every ride, every ball taken in. */
  endPlay() {
    if (this.riding) this.leaveRide(false);
    this.kicking = null;
    if (this.ballGuide) this.ballGuide.group.visible = false;
    for (const ball of this.balls.values()) this.takeIn(ball);
    this.balls.clear();
    this.ballHeld = null;
    this.hideBall();
    for (const s of this.settling) this.poseRig(s.it, s.rig, s.rest);
    this.settling.length = 0;
  },

  // ---------------------------------------------------------------- riding

  /**
   * Gets on a ride: over BLEND seconds the walker is carried from where they stand to
   * their place on it, and turned the way it faces.
   */
  board(it, entry) {
    const a = it.spec, rig = a.rigs[entry.rig], p = this.p;
    this.settling = this.settling.filter(s => !(s.it === it && s.rig === entry.rig));
    const ride = {
      it, entry, rig, angle: rig?.rest ?? 0, speed: 0, blend: 0, yawTo: null,
      from: { x: p.x, z: p.z, eye: p.feet + EYE },
    };
    if (entry.ride === 'spin') {
      // Where on the deck, as an angle round it in the deck's own turn.
      const local = inAmenity(it, a, v1.set(p.x, p.feet, p.z));
      ride.spot = Math.atan2(local[2] - entry.pivot[2], local[0] - entry.pivot[0]);
      ride.angle = 0;
    }
    // Facing along a swing and a rider (their model's +x), and along a seesaw to its
    // middle.
    if (entry.ride === 'swing' || (entry.ride === 'rock' && !entry.seat[0])) ride.yawTo = Math.atan2(-Math.cos(it.turn), Math.sin(it.turn));
    if (entry.ride === 'rock' && entry.seat[0]) {
      const middle = onAmenity(it, a, entry.pivot, v1), seat = onAmenity(it, a, [entry.pivot[0] + entry.seat[0], 0, entry.pivot[2]], v2);
      ride.yawTo = Math.atan2(-(middle.x - seat.x), -(middle.z - seat.z));
      ride.angle = -Math.sign(entry.seat[0]) * 0.12;
    }
    if (entry.ride === 'slide') Object.assign(ride, slideTrack(it, entry), { s: 0, sit: 0, rise: 0 });
    this.riding = ride;
    p.vy = 0;
    p.fly = false;
    this.fell = null;
    this.flung = null;
    this.eyeShift = 0;
  },

  /**
   * A frame on a ride, in place of walking (walk.js step): W pumps a swing, pushes a
   * roundabout, bounces a seesaw or a rider; S brakes; Space jumps off.
   */
  rideStep(deltaTime) {
    const r = this.riding, k = this.keys, e = r.entry, a = r.it.spec;
    const push = k.has('KeyW') || k.has('ArrowUp'), brake = k.has('KeyS') || k.has('ArrowDown');
    if (k.has('Space') && !r.jumping && r.blend >= 1) return this.leaveRide(true);
    if (!k.has('Space')) r.jumping = false;
    const dt = deltaTime;
    r.blend = Math.min(1, r.blend + dt / BLEND);
    if (e.ride === 'swing') {
      const length = e.length * a.scale;
      r.speed += (-(REAL_G / length) * Math.sin(r.angle) - 0.08 * r.speed) * dt;
      if (push && Math.abs(r.angle) < 1.1) r.speed += (r.speed >= 0 ? 1 : -1) * 1.6 * dt;
      if (brake) r.speed *= Math.max(0, 1 - 2 * dt);
      r.angle += r.speed * dt;
      if (Math.abs(r.angle) > 1.25) { r.angle = Math.sign(r.angle) * 1.25; r.speed *= -0.2; }
    } else if (e.ride === 'spin') {
      if (push) r.speed = Math.min(3.5, r.speed + 2.5 * dt);
      r.speed = Math.max(0, r.speed - (brake ? 2 : 0.35) * dt);
      r.angle += r.speed * dt;
      this.p.yaw += r.speed * dt; // turned with the deck
    } else if (e.ride === 'rock') {
      const seesaw = !!e.seat[0], down = seesaw ? -Math.sign(e.seat[0]) * 0.12 : 0;
      const stiff = seesaw ? 10 : 30, damp = seesaw ? 1.2 : 2;
      r.speed += (-stiff * (r.angle - down) - damp * r.speed) * dt;
      if (push && !r.pushed) {
        // Off the ground at the bottom of a seesaw's swing; a rider takes it any time.
        if (!seesaw || Math.abs(r.angle - down) < 0.04) r.speed += (seesaw ? Math.sign(e.seat[0]) * 2.5 : (r.speed >= 0 ? 1 : -1) * 1.5 + 0.5);
        r.pushed = true;
      }
      if (!push) r.pushed = false;
      r.angle += r.speed * dt;
      const reach = e.reach;
      if (Math.abs(r.angle) > reach) { r.angle = Math.sign(r.angle) * reach; r.speed *= -0.3; }
    } else if (e.ride === 'slide') {
      this.slideStep(r, dt);
      if (!this.riding) return;
    }
    this.poseRide(dt);
  },

  /**
   * A slide as one run: up the ladder facing it, across the platform sitting down by
   * its edge, down the chute gathering speed as its slope gives it, along the run-out
   * slowing, and up off the end onto the ground. Nothing is cut from one to the next.
   */
  slideStep(r, dt) {
    const at = r.marks, s = r.s;
    if (s < at.top) {
      r.speed = 0.5;
      r.yawTo = r.ladderYaw;
    } else if (s < at.edge) {
      r.speed = 0.6;
      r.yawTo = r.chuteYaw;
    } else if (s < at.foot) {
      const slope = r.slopeAt(s);
      r.speed = Math.max(0.35, r.speed + REAL_G * 1.6 * (Math.sin(slope) - 0.25 * Math.cos(slope)) * dt);
    } else r.speed = Math.max(0.15, r.speed - 2.5 * dt);
    if (s < at.end) r.s = Math.min(at.end, s + r.speed * dt);
    // Sat down across the platform, up again once off the end.
    r.sit = r.s < at.top ? 0 : r.s < at.edge ? smooth((r.s - at.top) / (at.edge - at.top)) : 1;
    if (r.s >= at.end) {
      r.rise = Math.min(1, r.rise + dt / 0.45);
      r.sit = 1 - smooth(r.rise);
      if (r.rise >= 1) {
        const p = this.p;
        this.riding = null;
        Object.assign(p, { feet: this.height(p.x, p.z), vy: 0, ground: true });
      }
    }
  },

  /**
   * Puts the walker where the ride has them - eased there from where they stood while
   * getting on - and the rig where it has got to.
   */
  poseRide(dt = 0) {
    const r = this.riding;
    if (!r) return;
    const e = r.entry, it = r.it, a = it.spec, p = this.p;
    let at, eye;
    if (e.ride === 'swing') {
      at = rigPoint(it, a, r.rig, r.angle, [0, -e.length, 0], v1);
      eye = at.y + SIT;
    } else if (e.ride === 'spin') {
      at = rigPoint(it, a, r.rig, r.angle, [Math.cos(r.spot) * e.r, e.floor, Math.sin(r.spot) * e.r], v1);
      eye = at.y + EYE;
    } else if (e.ride === 'rock') {
      at = rigPoint(it, a, r.rig, r.angle, e.seat, v1);
      eye = at.y + SIT;
    } else {
      at = r.pointAt(r.s, v1);
      eye = at.y + EYE + (SIT - EYE) * r.sit;
    }
    const k = smooth(r.blend);
    p.x = r.from.x + (at.x - r.from.x) * k;
    p.z = r.from.z + (at.z - r.from.z) * k;
    p.feet = r.from.eye + (eye - r.from.eye) * k - EYE;
    if (r.yawTo !== null && dt > 0) {
      const turn = Math.atan2(Math.sin(r.yawTo - p.yaw), Math.cos(r.yawTo - p.yaw));
      p.yaw += turn * Math.min(1, dt * 7);
      if (Math.abs(turn) < 1e-3 && e.ride !== 'slide') r.yawTo = null;
    }
    if (r.rig) this.poseRig(it, e.rig, r.angle);
  },

  /** Poses one rig of an amenity at `angle`. */
  poseRig(it, index, angle) {
    const rig = it.spec.rigs[index], drawn = it.rigs?.[index];
    if (!drawn) return;
    drawn.scatter.pose(drawn.item, rigMatrix(it, it.spec, rig, angle, m1));
  },

  /**
   * Gets off: `jump` with Space, carried on by what the ride was doing - flung off a
   * swing at the speed of its seat - and otherwise stepped off where it stands. The eye
   * eases from the seat to standing rather than jumping there (eyeShift).
   */
  leaveRide(jump) {
    const r = this.riding;
    if (!r) return;
    this.riding = null;
    const e = r.entry, a = r.it.spec, p = this.p, eye = p.feet + EYE;
    if (r.rig) this.settling.push({ it: r.it, rig: e.rig, angle: r.angle, speed: r.speed, rest: r.rig.rest ?? 0, kind: e.ride, entry: e });
    let vx = 0, vz = 0, vy = 0;
    if (e.ride === 'swing' && jump) {
      // The seat's speed, along the swing's x as the model has it.
      const speed = r.speed * e.length * a.scale, c = Math.cos(r.it.turn), s = Math.sin(r.it.turn);
      const along = Math.cos(r.angle) * speed;
      vx = along * c; vz = -along * s; vy = Math.sin(r.angle) * speed;
    } else if (e.ride === 'spin' && jump) {
      const c = onAmenity(r.it, a, e.pivot, v1), dx = p.x - c.x, dz = p.z - c.z;
      vx = -dz * r.speed; vz = dx * r.speed;
    }
    const seated = e.ride === 'swing' || e.ride === 'rock' || (e.ride === 'slide' && r.sit > 0.5);
    const ground = this.height(p.x, p.z, p.feet);
    p.feet = jump ? Math.max(ground, eye - SIT - 0.05) : e.ride === 'spin' ? p.feet : ground;
    this.eyeShift = eye - (p.feet + EYE);
    if (seated) this.unseat = 1;
    p.vy = jump ? Math.max(vy, 0) + 2.4 : 0;
    p.ground = !jump && p.feet <= ground;
    this.fell = jump ? p.feet : null;
    this.flung = Math.hypot(vx, vz) > 0.05 ? { x: vx, z: vz } : null;
  },

  /** Carries a walker flung off a ride on through the air, until they land (walk.js step). */
  coast(deltaTime) {
    const f = this.flung, p = this.p;
    if (!f) return;
    if (p.ground) { this.flung = null; return; }
    const nx = p.x + f.x * deltaTime, nz = p.z + f.z * deltaTime;
    if (this.height(nx, p.z, p.feet) <= p.feet + 0.05) p.x = nx; else f.x = 0;
    if (this.height(p.x, nz, p.feet) <= p.feet + 0.05) p.z = nz; else f.z = 0;
  },

  // Rides left moving swing on and come to rest by themselves.
  settle(deltaTime) {
    for (const s of this.settling) {
      if (s.kind === 'swing') {
        const length = s.entry.length * s.it.spec.scale;
        s.speed += (-(REAL_G / length) * Math.sin(s.angle) - 0.35 * s.speed) * deltaTime;
      } else if (s.kind === 'spin') s.speed = Math.max(0, s.speed - 0.5 * deltaTime);
      else s.speed += (-12 * (s.angle - s.rest) - 3 * s.speed) * deltaTime;
      s.angle += s.speed * deltaTime;
      const done = s.kind === 'spin' ? s.speed === 0 : Math.abs(s.angle - s.rest) < 0.002 && Math.abs(s.speed) < 0.01;
      this.poseRig(s.it, s.rig, done && s.kind !== 'spin' ? s.rest : s.angle);
      s.done = done;
    }
    if (this.settling.some(s => s.done)) this.settling = this.settling.filter(s => !s.done);
  },

  // ---------------------------------------------------------------- balls

  // Each court near enough has its ball out; one far enough is taken in.
  putOutBalls() {
    const p = this.p;
    for (const it of this.amenitiesNear(NEAR)) {
      for (const entry of it.spec.play) {
        // One ball to each court: keyed by the court, not by what kind of court it is.
        if (!entry.ball || this.balls.has(it)) continue;
        this.balls.set(it, this.makeBall(it, entry));
      }
    }
    for (const [it, ball] of this.balls) {
      if (ball === this.ballHeld) continue;
      if (Math.hypot(it.x - p.x, it.z - p.z) > FAR) {
        this.takeIn(ball);
        this.balls.delete(it);
      }
    }
  },

  makeBall(it, entry) {
    const kind = entry.ball, spec = BALLS[kind];
    const geo = ballGeometry(kind, this.scene.style, spec.r);
    const mesh = new THREE.Mesh(geo, this.scene.bendable(new THREE.MeshBasicMaterial({ vertexColors: true })));
    mesh.frustumCulled = false;
    this.scene.scene.add(mesh);
    const home = onAmenity(it, it.spec, entry.at, new THREE.Vector3());
    home.y = this.height(home.x, home.z, home.y + 0.5, POINT) + spec.r;
    const ball = { kind, it, entry, ...spec, mesh, home, pos: home.clone(), vel: new THREE.Vector3(), rest: true, lost: 0, played: null, lag: 0 };
    mesh.position.copy(ball.pos);
    return ball;
  },

  takeIn(ball) {
    ball.mesh.removeFromParent();
    ball.mesh.geometry.dispose();
    ball.mesh.material.dispose();
  },

  /**
   * A ball's flight, bounces and roll, a frame at a time, and what it does to its court.
   * Stepped STEP at a time however long the frame was, as the guide flies it, so the
   * ball goes where the guide said.
   */
  rollBall(ball, deltaTime) {
    ball.lag += deltaTime;
    while (ball.lag >= STEP) {
      this.ballStep(ball, STEP);
      ball.lag -= STEP;
    }
    this.standOff(ball);
    ball.mesh.position.copy(ball.pos);
    if (ball.vel.lengthSq() > 1e-6) {
      const turn = ball.vel.length() * deltaTime / ball.r;
      ball.mesh.rotateOnWorldAxis(v1.set(ball.vel.z, 0, -ball.vel.x).normalize(), turn);
    }
    // Lost - lying off its court, up on something, out over the water, or a long way
    // off - it is put back in the middle of its court.
    const a = ball.it.spec, local = inAmenity(ball.it, a, ball.pos), [x0, z0, x1, z1] = ball.entry.area;
    const off = Math.max(0, x0 - local[0], local[0] - x1, z0 - local[2], local[2] - z1) * a.scale;
    const up = ball.rest && this.height(ball.pos.x, ball.pos.z, ball.pos.y, POINT) > ball.it.y + 0.25;
    ball.lost = (off > 0.2 && ball.rest) || up || off > 4 ? ball.lost + deltaTime : 0;
    if (ball.lost > LOST || ball.pos.y < WATER || off > 12) this.putBack(ball);
  },

  /** A ball back in the middle of its court, still. */
  putBack(ball) {
    ball.pos.copy(ball.home);
    ball.vel.set(0, 0, 0);
    ball.lost = 0;
    ball.played = null;
    ball.inNet = null;
    ball.rest = true;
    if (this.kicking?.ball === ball) this.kicking = null;
  },

  ballStep(ball, h) {
    const pos = ball.pos, vel = ball.vel, it = ball.it, a = it.spec;
    const before = v2.copy(pos);
    vel.y -= BALL_G * h;
    pos.addScaledVector(vel, h);
    // Walls: back out along whichever way went in.
    if (this.boxAt(pos)?.kind === 'building') {
      if (!this.boxAt(v3.set(before.x, pos.y, pos.z))) { pos.x = before.x; vel.x *= -0.5; }
      else if (!this.boxAt(v3.set(pos.x, pos.y, before.z))) { pos.z = before.z; vel.z *= -0.5; }
      else { pos.copy(before); vel.multiplyScalar(-0.5); }
    }
    // Posts: a goal's, a hoop's pole, a swing's legs.
    for (const o of this.propGrid ? this.propsNear(pos.x, pos.z, this.ballNear ||= []) : []) {
      if (pos.y > o.y + 1.2) continue;
      const dx = pos.x - o.x, dz = pos.z - o.z, d = Math.hypot(dx, dz), want = o.r + ball.r;
      if (d >= want || d < 1e-6) continue;
      const nx = dx / d, nz = dz / d, into = vel.x * nx + vel.z * nz;
      pos.x = o.x + nx * want;
      pos.z = o.z + nz * want;
      if (into < 0) { vel.x -= 1.6 * into * nx; vel.z -= 1.6 * into * nz; }
    }
    const was = inAmenity(it, a, before), now = inAmenity(it, a, pos), k = a.scale;
    for (const e of a.play) {
      if (e.hoop) this.hoop(ball, e, was, now);
      else if (e.goal) this.goalLine(ball, e, was, now);
      else if (e.net) this.net(ball, e, was, now, k);
    }
    // The ground, under the ball's middle: a ball does not stand on a curb it is beside.
    const floor = this.height(pos.x, pos.z, pos.y, POINT);
    if (pos.y - ball.r < floor) {
      pos.y = floor + ball.r;
      if (vel.y < 0) {
        ball.touched = true;
        if (vel.y < -0.4) this.bounced(ball, now);
        vel.y = -vel.y * ball.bounce;
        if (vel.y < 0.15) vel.y = 0;
      }
      const keep = Math.exp(-ball.roll * h);
      vel.x *= keep;
      vel.z *= keep;
    }
    ball.rest = pos.y - ball.r - floor < 0.01 && vel.lengthSq() < 0.01;
  },

  // A rim: the ball off it like off a ring, and through it from above a basket.
  hoop(ball, e, was, now) {
    const a = ball.it.spec, k = a.scale;
    const c = onAmenity(ball.it, a, e.hoop, v3), R = e.r * k, pos = ball.pos;
    const dx = pos.x - c.x, dz = pos.z - c.z, flat = Math.hypot(dx, dz) || 1e-6;
    const qx = c.x + dx / flat * R, qz = c.z + dz / flat * R;
    const d = v4.set(pos.x - qx, pos.y - c.y, pos.z - qz), gap = d.length(), want = ball.r + 0.005 * k;
    if (gap < want && gap > 1e-6) {
      d.divideScalar(gap);
      pos.addScaledVector(d, want - gap);
      const into = ball.vel.dot(d);
      if (into < 0) ball.vel.addScaledVector(d, -1.6 * into);
      if (ball.played) ball.played.rim = true;
    }
    // The board.
    const [bx0, by0, bz0] = e.board.from, [bx1, by1, bz1] = e.board.to;
    const inside = (v, lo, hi, r) => v > Math.min(lo, hi) - r && v < Math.max(lo, hi) + r;
    const rl = ball.r / k;
    if (inside(now[0], bx0, bx1, rl) && inside(now[1], by0, by1, rl) && inside(now[2], bz0, bz1, rl)) {
      const back = Math.sign(was[0] - (bx0 + bx1) / 2) || 1, n = v4.set(back * Math.cos(ball.it.turn), 0, -back * Math.sin(ball.it.turn));
      const into = ball.vel.dot(n);
      if (into < 0) ball.vel.addScaledVector(n, -1.7 * into);
      pos.addScaledVector(n, 0.01);
    }
    // In: down through the ring, from above it.
    if (was[1] > e.hoop[1] && now[1] <= e.hoop[1] && Math.hypot(now[0] - e.hoop[0], now[2] - e.hoop[2]) < e.r - rl * 0.4) {
      const swish = ball.played && !ball.played.rim;
      if (!ball.ghost) this.flash(swish ? 'Swish! Two points' : 'In off the rim - two points');
      if (ball.played) ball.played.scored = true;
    }
  },

  // A goal: the ball over its line between the posts and under the bar is in, and the
  // net holds it.
  goalLine(ball, e, was, now) {
    const a = ball.it.spec, x = e.from[0], side = e.goal, r = ball.r / a.scale;
    if ((was[0] - x) * side < 0 && (now[0] - x) * side >= 0 && now[2] > e.from[2] + r && now[2] < e.to[2] - r && now[1] < e.to[1]) {
      if (ball.inNet !== e && !ball.ghost) this.flash('Goal!');
      ball.inNet = e;
    }
    if (ball.inNet !== e) return;
    const depth = (now[0] - x) * side;
    if (depth < -0.02) { ball.inNet = null; return; }
    if (depth > NET_DEPTH - r) {
      ball.pos.copy(onAmenity(ball.it, a, [x + side * (NET_DEPTH - r), now[1], now[2]], v3));
      const c = Math.cos(ball.it.turn), s = Math.sin(ball.it.turn), along = ball.vel.x * c - ball.vel.z * s;
      if (along * side > 0) { ball.vel.x -= 1.3 * along * c; ball.vel.z += 1.3 * along * s; }
      ball.vel.multiplyScalar(0.5);
    }
  },

  // The net across a volleyball court: it stops a ball that meets it, and one that
  // goes over and comes down in the court is in.
  net(ball, e, was, now, k) {
    const r = ball.r / k;
    const crossed = Math.sign(was[0]) !== Math.sign(now[0]) && now[2] > e.from[2] && now[2] < e.to[2];
    if (!crossed) return;
    if (now[1] < e.to[1] + r && now[1] > e.from[1] - r) {
      const c = Math.cos(ball.it.turn), s = Math.sin(ball.it.turn);
      const along = ball.vel.x * c - ball.vel.z * s; // along the court's x
      ball.vel.x -= 1.8 * along * c;
      ball.vel.z += 1.8 * along * s;
      ball.pos.copy(onAmenity(ball.it, ball.it.spec, [Math.sign(was[0]) * (r + 0.002), now[1], now[2]], v3));
      if (ball.played) {
        ball.played.net = true;
        if (!ball.ghost) this.flash('Into the net');
      }
    } else if (ball.played) ball.played.over = Math.sign(now[0]);
  },

  // A ball coming down hard: where it lands decides a serve.
  bounced(ball, now) {
    const play = ball.played;
    if (!play || play.landed) return;
    play.landed = true;
    if (ball.kind !== 'volley' || play.net || !play.over) return;
    const [x0, z0, x1, z1] = ball.entry.area;
    const r = ball.r / ball.it.spec.scale;
    const inside = now[0] > x0 - r && now[0] < x1 + r && now[2] > z0 - r && now[2] < z1 + r && Math.sign(now[0]) === play.over;
    if (!ball.ghost) this.flash(inside ? (play.jump ? 'Ace! A jump serve in' : 'In!') : 'Out');
  },

  // Walked into, a ball stays where it is and the walker stops against it: a ball
  // moves only when it is played.
  standOff(ball) {
    const p = this.p, dx = p.x - ball.pos.x, dz = p.z - ball.pos.z, d = Math.hypot(dx, dz), want = BODY + ball.r;
    if (d >= want || ball.pos.y - p.feet > 0.25 || this.riding) return;
    const nx = d > 1e-6 ? dx / d : Math.sin(p.yaw), nz = d > 1e-6 ? dz / d : Math.cos(p.yaw);
    p.x = ball.pos.x + nx * want;
    p.z = ball.pos.z + nz * want;
  },

  /** A click on a ball at hand: a soccer ball is kicked, a volleyball in the air hit, the others picked up. */
  touchBall(ball) {
    if (ball.kind === 'soccer') {
      // The leg is drawn back first (legs.js); the ball goes when the foot meets it.
      this.kicked = performance.now();
      this.kicking = { ball, left: KICK_LAG };
      return;
    }
    if (ball.kind === 'volley' && !ball.rest && ball.pos.y - this.p.feet > 0.2) return this.send(ball, 'hit');
    this.ballHeld = ball;
    ball.mesh.visible = false;
    ball.vel.set(0, 0, 0);
    this.showBall(ball);
  },

  /** Lets go of the ball held, at the walker's feet. */
  dropBall() {
    const ball = this.ballHeld;
    if (!ball) return;
    this.ballHeld = null;
    this.hideBall();
    ball.mesh.visible = true;
    ball.pos.set(this.p.x - Math.sin(this.p.yaw) * 0.2, this.p.feet + 0.3, this.p.z - Math.cos(this.p.yaw) * 0.2);
    ball.vel.set(0, 0, 0);
  },

  /** The ball held, let go: shot, served or thrown (plan). */
  throwBall(ball) {
    this.ballHeld = null;
    this.hideBall();
    ball.mesh.visible = true;
    this.send(ball, ball.kind === 'volley' ? 'serve' : 'throw');
  },

  /** Sets `ball` off as `how` plans it: from where it leaves, at the speed the plan gives it. */
  send(ball, how) {
    const plan = this.plan(ball, how);
    ball.pos.copy(plan.from);
    ball.vel.copy(plan.vel);
    ball.lag = 0;
    ball.inNet = null;
    ball.played = { rim: false, net: false, over: 0, landed: false, jump: !this.p.ground };
  },

  /**
   * Where `ball` would leave from and how fast, for `how`: straight on from where the
   * walker looks, lifted by the throw - 'throw' (a basketball's shot), 'serve' (a
   * volleyball, standing or, flatter and harder, jumping), 'hit' (a volleyball in the
   * air) or 'kick' (a football, harder running): { from, vel }. Nothing is aimed for
   * them; the guide shows where it goes (drawBallGuide), and it goes there.
   */
  plan(ball, how) {
    const p = this.p, from = how === 'kick'
      ? new THREE.Vector3(ball.pos.x, this.height(ball.pos.x, ball.pos.z, ball.pos.y, POINT) + ball.r + 0.005, ball.pos.z)
      : this.releasePoint(ball, new THREE.Vector3());
    const run = this.keys.has('ShiftLeft') || this.keys.has('ShiftRight');
    const [lift, speed] = how === 'throw' ? [0.45, 3]
      : how === 'kick' ? [0.3, run ? 6 : 4.2]
        : how === 'hit' ? [0.8, 2.6]
          : p.ground ? [0.5, 3.6] : [0.15, 5];
    const up = Math.max(-0.4, Math.min(1.35, p.pitch + lift));
    const vel = new THREE.Vector3(-Math.sin(p.yaw) * Math.cos(up), Math.sin(up), -Math.cos(p.yaw) * Math.cos(up)).multiplyScalar(speed);
    return { from, vel };
  },

  /**
   * The path `plan` would send `ball` on, flown by the ball's own steps (ballStep) until
   * it first comes down, or for FLIGHT seconds: { path, point, played } - played being
   * what it did on the way: through the hoop (scored), into a goal (goal), over a net.
   */
  flight(ball, plan) {
    const ghost = this.ghost ||= { pos: new THREE.Vector3(), vel: new THREE.Vector3(), ghost: true };
    Object.assign(ghost, {
      kind: ball.kind, it: ball.it, entry: ball.entry, r: ball.r, bounce: ball.bounce, roll: ball.roll,
      touched: false, inNet: null, played: { rim: false, net: false, over: 0, landed: false },
    });
    ghost.pos.copy(plan.from);
    ghost.vel.copy(plan.vel);
    const path = [plan.from.clone()];
    for (let n = 1; n * STEP <= FLIGHT && !ghost.touched; n++) {
      this.ballStep(ghost, STEP);
      if (n % 2 === 0 || ghost.touched) path.push(ghost.pos.clone());
    }
    return { path, point: ghost.pos.clone().addScaledVector(UP, -ball.r), played: { ...ghost.played, goal: !!ghost.inNet } };
  },

  /** What ball the walker is about to send, and how - for the guide - or null. */
  aiming() {
    if (!this.freeHands() || this.riding || this.still) return null;
    if (this.kicking) return { ball: this.kicking.ball, how: 'kick' };
    if (this.ballHeld) return { ball: this.ballHeld, how: this.ballHeld.kind === 'volley' ? 'serve' : 'throw' };
    const what = this.playable();
    if (what?.kind !== 'ball') return null;
    if (what.ball.kind === 'soccer') return { ball: what.ball, how: 'kick' };
    if (what.ball.kind === 'volley' && !what.ball.rest && what.ball.pos.y - this.p.feet > 0.2) return { ball: what.ball, how: 'hit' };
    return null;
  },

  /**
   * The guide for a ball about to be sent: the same line and marker the tools are
   * aimed by (trajectory.js), along the path it would fly.
   *
   * Implements: REQ-WALK-058
   */
  drawBallGuide() {
    const aim = this.aiming();
    if (!aim && !this.ballGuide) return;
    const guide = this.ballGuide ||= makeGuide(this.scene, undefined, { spacing: 0.05, fadeIn: 0.3 });
    if (!aim) { guide.group.visible = false; return; }
    const { path, point } = this.flight(aim.ball, this.plan(aim.ball, aim.how));
    guide.draw(path, point, UP, this.scene.walkCamera.position);
  },

  // Where a ball leaves the hands: in front of the eye and a little under it, or over
  // the head for a jump serve.
  releasePoint(ball, out) {
    const p = this.p, up = ball.kind === 'volley' && !p.ground ? 0.12 : -0.08;
    return out.set(p.x - Math.sin(p.yaw) * 0.18, p.feet + EYE + up, p.z - Math.cos(p.yaw) * 0.18);
  },

  /** The ball held, in both hands, in front of the camera. */
  showBall(ball) {
    this.hideBall();
    const cam = this.scene.walkCamera;
    this.scene.viewScene.add(cam);
    cam.add(this.lights ||= viewLights());
    const mesh = new THREE.Mesh(ballGeometry(ball.kind, this.scene.style, HELD_BALL),
      new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.7 }));
    this.ballView = new THREE.Group();
    this.ballView.scale.setScalar(0.5);
    this.ballView.add(ballInHands(mesh));
    cam.add(this.ballView);
  },

  hideBall() {
    if (!this.ballView) return;
    this.ballView.removeFromParent();
    this.ballView.traverse(o => { if (o.isMesh && !o.isSkinnedMesh) { o.geometry.dispose(); o.material.dispose(); } });
    this.ballView = null;
  },
};

const smooth = t => t * t * (3 - 2 * t);

/**
 * A slide's path in the world, for riding by distance along it: its points, where the
 * ladder's top, the platform's edge, the chute's foot and the end are along it
 * (marks), which way the ladder and the chute face, and the point and the slope at
 * any distance.
 */
function slideTrack(it, entry) {
  const points = entry.path.map(at => onAmenity(it, it.spec, at, new THREE.Vector3()));
  const along = [0];
  for (let i = 1; i < points.length; i++) along.push(along[i - 1] + points[i].distanceTo(points[i - 1]));
  const segment = s => {
    let i = 1;
    while (i < points.length - 1 && along[i] < s) i++;
    return i;
  };
  const facing = (a, b) => Math.atan2(-(b.x - a.x), -(b.z - a.z));
  const last = points.length - 1;
  return {
    marks: { top: along[1], edge: along[entry.sit], foot: along[last - 1], end: along[last] },
    ladderYaw: facing(points[0], points[1]),
    chuteYaw: facing(points[entry.sit], points[last - 1]),
    pointAt(s, out) {
      const i = segment(s), u = (s - along[i - 1]) / Math.max(1e-6, along[i] - along[i - 1]);
      return out.copy(points[i - 1]).lerp(points[i], Math.max(0, Math.min(1, u)));
    },
    slopeAt(s) {
      const i = segment(s), a = points[i - 1], b = points[i];
      return Math.atan2(a.y - b.y, Math.hypot(b.x - a.x, b.z - a.z));
    },
  };
}

// A rig's point `local` (from its pivot) where the rig is turned `angle`.
function rigPoint(it, a, rig, angle, local, out) {
  return out.set(...local).applyMatrix4(rigMatrix(it, a, rig, angle, m1));
}

// A ball, colored as the style has it: a soccer ball's black pentagons, a
// basketball's seams, a volleyball's panels; a ball of solder on a board; an orb in
// the galaxy.
function ballGeometry(kind, style, r) {
  const geo = shaded(kind === 'soccer' ? new THREE.IcosahedronGeometry(r, 1) : new THREE.SphereGeometry(r, 28, 18));
  const position = geo.getAttribute('position'), color = geo.getAttribute('color');
  const c = new THREE.Color(), corners = new THREE.IcosahedronGeometry(1, 0).getAttribute('position');
  for (let i = 0; i < position.count; i += 3) {
    const cx = (position.getX(i) + position.getX(i + 1) + position.getX(i + 2)) / 3 / r;
    const cy = (position.getY(i) + position.getY(i + 1) + position.getY(i + 2)) / 3 / r;
    const cz = (position.getZ(i) + position.getZ(i + 1) + position.getZ(i + 2)) / 3 / r;
    if (style === 'circuit') c.set('#d4d9de');
    else if (style === 'galaxy') c.set(kind === 'soccer' ? '#6ff4ff' : kind === 'basket' ? '#ffd27a' : '#c9b4ff');
    else if (kind === 'soccer') {
      let near = false;
      for (let k = 0; k < corners.count && !near; k++) {
        const x = corners.getX(k), y = corners.getY(k), z = corners.getZ(k), n = Math.hypot(x, y, z);
        near = (cx * x + cy * y + cz * z) / n / Math.hypot(cx, cy, cz) > 0.93;
      }
      c.set(near ? '#1d1f22' : '#f4f4f0');
    } else if (kind === 'basket') {
      c.set('#d9692b');
    } else {
      const ax = Math.abs(cx), ay = Math.abs(cy), az = Math.abs(cz);
      c.set(ax > ay && ax > az ? '#f4f4f0' : ay > az ? '#f2c230' : '#2a5db0');
    }
    for (let k = i; k < i + 3; k++) color.setXYZ(k, color.getX(k) * c.r * 1.25, color.getY(k) * c.g * 1.25, color.getZ(k) * c.b * 1.25);
  }
  if (kind !== 'basket' || style !== 'city') return geo;
  // A basketball's seams: two great circles across each other and a ring either side.
  const seam = (geometry) => {
    const g = shaded(geometry), dark = g.getAttribute('color');
    for (let k = 0; k < dark.count; k++) dark.setXYZ(k, dark.getX(k) * 0.16, dark.getY(k) * 0.1, dark.getZ(k) * 0.08);
    return g;
  };
  const tube = r * 0.028;
  const ball = merge([
    geo,
    seam(new THREE.TorusGeometry(r * 1.004, tube, 4, 48)),
    seam(new THREE.TorusGeometry(r * 1.004, tube, 4, 48).rotateY(Math.PI / 2)),
    ...[-1, 1].map(side => seam(new THREE.TorusGeometry(r * 1.004 * Math.sqrt(1 - 0.62 ** 2), tube, 4, 48).rotateX(Math.PI / 2).translate(0, side * r * 0.62, 0))),
  ]);
  ball.computeVertexNormals(); // the one held is lit (showBall)
  return ball;
}

export { BALLS, ballGeometry };
