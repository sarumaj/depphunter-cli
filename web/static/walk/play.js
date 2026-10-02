// Playing on what the parks hold (map/amenities.js), with empty hands: riding a swing,
// a roundabout, a seesaw, a spring rider or a slide, and playing ball - a shot at a
// hoop, a kick at a goal, a serve over a net. Mixed into Walker (walk.js).
//
// Only with nothing in either hand: tools put away (H) and nothing carried in the
// left. A walker holding a tool is told how to free their hands instead.
//
// A ride moves the walker and the one rig being ridden (map/lod.js pose); everything
// else in the park stays as it is drawn. A ball is the one thing out there with
// physics of its own: it falls, bounces, rolls and is pushed about by whoever walks
// into it, and it knows the hoops, goals and net of its own court.
//
// Implements: REQ-WALK-057, REQ-WALK-058

import * as THREE from '../vendor/three.module.min.js';
import { onAmenity, inAmenity, rigMatrix } from '../map/amenities.js';
import { shaded } from '../map/shapes.js';
import { EYE, WATER } from './walkbase.js';
import { ballInHands, viewLights, HELD_BALL } from './tools.js';

const RIDE_REACH = 0.5;     // how near a seat, a deck or a ladder has to be to get on
const SIT = 0.22;           // the eye over a seat
const REAL_G = 2.76;        // gravity in units, a unit being 3.55 m: what a swing swings by
const BALL_G = 4;           // ... and a ball falls by, a little brisker than the truth
const PICK = 0.45;          // how near a ball has to be to pick up, or to kick
const BODY = 0.12;          // the walker's radius, as walk.js has it
const NEAR = 14, FAR = 26;  // a court's ball is put out within NEAR, and taken in past FAR
const STEP = 1 / 120;       // a ball's physics step
const NET_DEPTH = 0.08;     // how far back of its line a goal's net holds a ball, as the model has it

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
      if ((d < PICK && low && this.facing(ball.pos) > 0.2) || inAir) {
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
    for (const ball of this.balls.values()) if (ball !== this.ballHeld) this.rollBall(ball, deltaTime);
    this.settle(deltaTime);
    if (this.riding) this.poseRide();
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
    for (const ball of this.balls.values()) this.takeIn(ball);
    this.balls.clear();
    this.ballHeld = null;
    this.hideBall();
    for (const s of this.settling) this.poseRig(s.it, s.rig, s.rest);
    this.settling.length = 0;
  },

  // ---------------------------------------------------------------- riding

  /** Gets on a ride. */
  board(it, entry) {
    const a = it.spec, rig = a.rigs[entry.rig];
    this.settling = this.settling.filter(s => !(s.it === it && s.rig === entry.rig));
    const ride = { it, entry, rig, angle: rig?.rest ?? 0, speed: 0, stage: 0, along: 0 };
    if (entry.ride === 'spin') {
      // Where on the deck, as an angle round it in the deck's own turn.
      const local = inAmenity(it, a, v1.set(this.p.x, this.p.feet, this.p.z));
      ride.spot = Math.atan2(local[2] - entry.pivot[2], local[0] - entry.pivot[0]);
      ride.angle = 0;
    }
    if (entry.ride === 'swing') this.p.yaw = Math.atan2(-Math.cos(it.turn), Math.sin(it.turn)); // facing along the swing: its model's +x
    if (entry.ride === 'rock') ride.angle = entry.seat[0] ? -Math.sign(entry.seat[0]) * 0.12 : 0;
    this.riding = ride;
    this.p.vy = 0;
    this.p.fly = false;
    this.fell = null;
    this.flung = null;
    this.poseRide();
  },

  /**
   * A frame on a ride, in place of walking (walk.js step): W pumps a swing, pushes a
   * roundabout, bounces a seesaw or a rider; S brakes; Space jumps off.
   */
  rideStep(deltaTime) {
    const r = this.riding, k = this.keys, e = r.entry, a = r.it.spec;
    const push = k.has('KeyW') || k.has('ArrowUp'), brake = k.has('KeyS') || k.has('ArrowDown');
    if (k.has('Space') && !r.jumping) return this.leaveRide(true);
    if (!k.has('Space')) r.jumping = false;
    const dt = deltaTime;
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
    }
    this.poseRide();
  },

  // Up the ladder, along the platform, down the chute and off the end.
  slideStep(r, dt) {
    const path = r.entry.path, a = r.it.spec;
    const from = onAmenity(r.it, a, path[r.stage], v1), to = onAmenity(r.it, a, path[r.stage + 1], v2);
    const length = from.distanceTo(to);
    const chute = r.stage === 3;
    if (chute) {
      const slope = Math.asin(Math.min(1, (from.y - to.y) / Math.max(1e-6, length)));
      r.speed = Math.max(0.3, r.speed + (REAL_G * 1.6 * (Math.sin(slope) - 0.25 * Math.cos(slope))) * dt);
    } else r.speed = r.stage === 4 ? Math.max(0.2, r.speed - 2 * dt) : r.stage === 0 ? 0.55 : 0.8;
    r.along += r.speed * dt;
    if (r.along >= length) {
      r.along -= length;
      r.stage++;
      if (r.stage === 3) {
        r.speed = 0.4;
        const next = onAmenity(r.it, a, path[4], v3);
        this.p.yaw = Math.atan2(-(next.x - to.x), -(next.z - to.z));
      }
      if (r.stage >= path.length - 1) {
        const end = onAmenity(r.it, a, path[path.length - 1], v3);
        this.riding = null;
        Object.assign(this.p, { x: end.x, z: end.z, feet: this.height(end.x, end.z), vy: 0, ground: true });
      }
    }
  },

  /** Puts the walker where the ride has them, and the rig where it has got to. */
  poseRide() {
    const r = this.riding;
    if (!r) return;
    const e = r.entry, it = r.it, a = it.spec, p = this.p;
    let at;
    if (e.ride === 'swing') {
      at = rigPoint(it, a, r.rig, r.angle, [0, -e.length, 0], v1);
      p.feet = at.y + SIT - EYE;
    } else if (e.ride === 'spin') {
      const local = [Math.cos(r.spot) * e.r, e.floor, Math.sin(r.spot) * e.r];
      at = rigPoint(it, a, r.rig, r.angle, local, v1);
      p.feet = at.y;
    } else if (e.ride === 'rock') {
      at = rigPoint(it, a, r.rig, r.angle, e.seat, v1);
      p.feet = at.y + SIT - EYE;
    } else {
      const path = e.path;
      if (r.stage >= path.length - 1) return;
      const from = onAmenity(it, a, path[r.stage], v1), to = onAmenity(it, a, path[r.stage + 1], v2);
      at = from.lerp(to, Math.min(1, r.along / Math.max(1e-6, from.distanceTo(to))));
      p.feet = r.stage >= 3 ? at.y + SIT - EYE : at.y;
    }
    p.x = at.x;
    p.z = at.z;
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
   * swing at the speed of its seat - and otherwise stepped off where it stands.
   */
  leaveRide(jump) {
    const r = this.riding;
    if (!r) return;
    this.riding = null;
    const e = r.entry, a = r.it.spec, p = this.p;
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
    if (e.ride !== 'spin' && e.ride !== 'slide') p.feet = Math.max(this.height(p.x, p.z), p.feet + EYE - SIT - 0.05);
    p.vy = jump ? Math.max(vy, 0) + 2.4 : 0;
    p.ground = false;
    this.fell = p.feet;
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
        if (!entry.ball || this.balls.has(entry)) continue;
        this.balls.set(entry, this.makeBall(it, entry));
      }
    }
    for (const [entry, ball] of this.balls) {
      if (ball === this.ballHeld) continue;
      if (Math.hypot(ball.it.x - p.x, ball.it.z - p.z) > FAR) {
        this.takeIn(ball);
        this.balls.delete(entry);
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
    home.y += spec.r;
    const ball = { kind, it, entry, ...spec, mesh, home, pos: home.clone(), vel: new THREE.Vector3(), rest: true, lost: 0, played: null };
    mesh.position.copy(ball.pos);
    return ball;
  },

  takeIn(ball) {
    ball.mesh.removeFromParent();
    ball.mesh.geometry.dispose();
    ball.mesh.material.dispose();
  },

  /** A ball's flight, bounces and roll, a frame at a time, and what it does to its court. */
  rollBall(ball, deltaTime) {
    const n = Math.max(1, Math.round(deltaTime / STEP)), h = deltaTime / n;
    for (let i = 0; i < n; i++) this.ballStep(ball, h);
    this.pushedBy(ball);
    ball.mesh.position.copy(ball.pos);
    if (ball.vel.lengthSq() > 1e-6) {
      const turn = ball.vel.length() * deltaTime / ball.r;
      ball.mesh.rotateOnWorldAxis(v1.set(ball.vel.z, 0, -ball.vel.x).normalize(), turn);
    }
    // Lost: out of the court for a while, or in the water. Back to the middle.
    const local = inAmenity(ball.it, ball.it.spec, ball.pos), [x0, z0, x1, z1] = ball.entry.area;
    const outside = local[0] < x0 - 0.4 || local[0] > x1 + 0.4 || local[2] < z0 - 0.4 || local[2] > z1 + 0.4;
    ball.lost = outside && ball.rest ? ball.lost + deltaTime : 0;
    if (ball.lost > 4 || ball.pos.y < WATER) {
      ball.pos.copy(ball.home);
      ball.vel.set(0, 0, 0);
      ball.lost = 0;
      ball.played = null;
    }
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
    // The ground.
    const floor = this.height(pos.x, pos.z, pos.y);
    if (pos.y - ball.r < floor) {
      pos.y = floor + ball.r;
      if (vel.y < 0) {
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
      this.flash(swish ? 'Swish! Two points' : 'In off the rim - two points');
      if (ball.played) ball.played.scored = true;
    }
  },

  // A goal: the ball over its line between the posts and under the bar is in, and the
  // net holds it.
  goalLine(ball, e, was, now) {
    const a = ball.it.spec, x = e.from[0], side = e.goal, r = ball.r / a.scale;
    if ((was[0] - x) * side < 0 && (now[0] - x) * side >= 0 && now[2] > e.from[2] + r && now[2] < e.to[2] - r && now[1] < e.to[1]) {
      if (ball.inNet !== e) this.flash('Goal!');
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
      if (ball.played) { ball.played.net = true; this.flash('Into the net'); }
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
    this.flash(inside ? (play.jump ? 'Ace! A jump serve in' : 'In!') : 'Out');
  },

  // Walked into, a ball is pushed ahead; a soccer ball is dribbled.
  pushedBy(ball) {
    const p = this.p, dx = ball.pos.x - p.x, dz = ball.pos.z - p.z, d = Math.hypot(dx, dz), want = BODY + ball.r;
    if (d >= want || ball.pos.y - p.feet > 0.25 || this.riding) return;
    const nx = d > 1e-6 ? dx / d : -Math.sin(p.yaw), nz = d > 1e-6 ? dz / d : -Math.cos(p.yaw);
    ball.pos.x = p.x + nx * want;
    ball.pos.z = p.z + nz * want;
    const speed = Math.max(0.6, Math.hypot(this.moving.x, this.moving.z) * 1.15);
    ball.vel.x = nx * speed;
    ball.vel.z = nz * speed;
  },

  /** A click on a ball at hand: a soccer ball is kicked, a volleyball in the air hit, the others picked up. */
  touchBall(ball) {
    if (ball.kind === 'soccer') return this.kick(ball);
    if (ball.kind === 'volley' && !ball.rest && ball.pos.y - this.p.feet > 0.2) return this.hit(ball, 0.8);
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

  /**
   * The ball held, let go: at a hoop of its court in view a shot is put up at it, a
   * volleyball is served over its net, and anything else is thrown where the walker
   * looks.
   */
  throwBall(ball) {
    this.ballHeld = null;
    this.hideBall();
    ball.mesh.visible = true;
    const p = this.p, from = this.releasePoint(ball, v1);
    ball.pos.copy(from);
    ball.played = { rim: false, net: false, over: 0, landed: false, jump: !p.ground };
    if (ball.kind === 'basket') {
      const hoop = this.target(ball, e => e.hoop, e => onAmenity(ball.it, ball.it.spec, e.hoop, new THREE.Vector3()), 0.5, 7);
      if (hoop) {
        // Each meter out puts a little more off the middle of the rim.
        const d = Math.hypot(hoop.x - from.x, hoop.z - from.z), spread = 0.004 + 0.009 * d;
        hoop.x += (Math.random() - 0.5) * 2 * spread;
        hoop.z += (Math.random() - 0.5) * 2 * spread;
        hoop.y += ball.r * 0.4;
        const line = Math.atan2(hoop.y - from.y, d);
        if (this.lob(ball, from, hoop, line + (Math.PI / 2 - line) * 0.5)) return;
      }
    }
    if (ball.kind === 'volley') return this.hit(ball, p.ground ? 0.5 : 0.05);
    ball.vel.set(-Math.sin(p.yaw) * Math.cos(p.pitch), Math.sin(p.pitch) + 0.25, -Math.cos(p.yaw) * Math.cos(p.pitch)).normalize().multiplyScalar(2.6);
  },

  /**
   * A volleyball hit over its net into the other half, 60% of the way into it where the
   * walker's look crosses, on the lowest arc from `rise` radians up that clears the
   * net; looking away from the net, it goes where they look.
   */
  hit(ball, rise) {
    const p = this.p, a = ball.it.spec, from = this.releasePoint(ball, new THREE.Vector3());
    ball.pos.copy(from);
    ball.played ||= { rim: false, net: false, over: 0, landed: false, jump: !p.ground };
    const net = a.play.find(e => e.net);
    const here = inAmenity(ball.it, a, from);
    const ahead = inAmenity(ball.it, a, v2.set(from.x - Math.sin(p.yaw), from.y, from.z - Math.cos(p.yaw)));
    const lx = ahead[0] - here[0], lz = ahead[2] - here[2], side = -Math.sign(here[0]) || 1;
    if (net && lx * side > 0.3 * Math.hypot(lx, lz)) {
      const [, z0, x1, z1] = ball.entry.area, depth = side * x1 * 0.6;
      const z = Math.max(z0 * 0.85, Math.min(z1 * 0.85, here[2] + lz * (depth - here[0]) / lx));
      const to = onAmenity(ball.it, a, [depth, 0, z], new THREE.Vector3());
      to.y += ball.r;
      const over = { at: -here[0] / (depth - here[0]), y: ball.it.y + net.to[1] * a.scale + ball.r * 1.5 };
      for (let angle = rise; angle < 1.3; angle += 0.04) if (this.lob(ball, from, to, angle, over)) return;
    }
    const lift = rise + Math.max(0, p.pitch);
    ball.vel.set(-Math.sin(p.yaw) * Math.cos(lift), Math.sin(lift), -Math.cos(p.yaw) * Math.cos(lift)).multiplyScalar(3);
  },

  /** Kicks a soccer ball where the walker looks, or at the goal when it is near where they look. */
  kick(ball) {
    const p = this.p, run = this.keys.has('ShiftLeft') || this.keys.has('ShiftRight');
    this.kicked = performance.now();
    ball.played = { rim: false, net: false, over: 0, landed: false, jump: false };
    ball.pos.y = Math.max(ball.pos.y, this.height(ball.pos.x, ball.pos.z) + ball.r + 0.005);
    const goal = this.target(ball, e => e.goal, e => onAmenity(ball.it, ball.it.spec, [e.from[0], e.to[1] * 0.4, (e.from[2] + e.to[2]) / 2], new THREE.Vector3()), 0.3, 14);
    if (goal) {
      const d = Math.hypot(goal.x - ball.pos.x, goal.z - ball.pos.z), spread = 0.01 + 0.012 * d;
      goal.x += (Math.random() - 0.5) * 2 * spread * Math.abs(Math.cos(ball.it.turn));
      goal.z += (Math.random() - 0.5) * 2 * spread * Math.abs(Math.sin(ball.it.turn));
      if (this.lob(ball, ball.pos.clone(), goal, run ? 0.12 : 0.25)) return;
    }
    const speed = run ? 6 : 4, lift = Math.max(0.05, Math.min(0.7, p.pitch + 0.25));
    ball.vel.set(-Math.sin(p.yaw) * Math.cos(lift), Math.sin(lift), -Math.cos(p.yaw) * Math.cos(lift)).multiplyScalar(speed);
  },

  /**
   * The one of `pick`ed play entries of a ball's court the walker is looking at, within
   * `cone` radians and `range` units: where `at` puts it, or null.
   */
  target(ball, pick, at, cone, range) {
    const p = this.p;
    let best = null, bestOff = cone;
    for (const e of ball.it.spec.play.filter(pick)) {
      const w = at(e), dx = w.x - p.x, dz = w.z - p.z, d = Math.hypot(dx, dz);
      if (d > range) continue;
      const off = Math.acos(Math.max(-1, Math.min(1, this.facing(w))));
      if (off < bestOff) { bestOff = off; best = w; }
    }
    return best;
  },

  /**
   * Sets a ball flying from `from` to land on `to`, launched `angle` radians up. With
   * `over` ({ at, y }), only if it is above `y` at the share `at` of the way there - a
   * net. Whether it was set flying: not when no speed it could be given gets it there.
   */
  lob(ball, from, to, angle, over = null) {
    const dx = to.x - from.x, dz = to.z - from.z, d = Math.hypot(dx, dz), dy = to.y - from.y;
    const denominator = 2 * Math.cos(angle) ** 2 * (d * Math.tan(angle) - dy);
    if (d < 1e-3 || denominator <= 0) return false;
    const v = Math.sqrt(BALL_G * d * d / denominator);
    if (!Number.isFinite(v) || v > 9) return false;
    if (over) {
      const t = over.at * d / (v * Math.cos(angle));
      if (from.y + v * Math.sin(angle) * t - BALL_G * t * t / 2 < over.y) return false;
    }
    ball.vel.set(dx / d * v * Math.cos(angle), v * Math.sin(angle), dz / d * v * Math.cos(angle));
    return true;
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
      const seam = Math.abs(cx) < 0.035 || Math.abs(cz) < 0.035 || Math.abs(Math.abs(cy) - 0.62) < 0.03;
      c.set(seam ? '#2a1a12' : '#d9692b');
    } else {
      const ax = Math.abs(cx), ay = Math.abs(cy), az = Math.abs(cz);
      c.set(ax > ay && ax > az ? '#f4f4f0' : ay > az ? '#f2c230' : '#2a5db0');
    }
    for (let k = i; k < i + 3; k++) color.setXYZ(k, color.getX(k) * c.r * 1.25, color.getY(k) * c.g * 1.25, color.getZ(k) * c.b * 1.25);
  }
  return geo;
}

export { BALLS, ballGeometry };
