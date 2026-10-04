// The walker under a parachute: opening it, gliding under it, landing and what the
// landing costs, cutting it away, and the canopies left lying about or drifting off.
// The canopy's own flight and rig are parachute.js's. Mixed into Walker (walk.js).

import * as THREE from '../vendor/three.module.min.js';
import { EYE, STEP, WATER, reducedMotion } from './walkbase.js';
import { toolFor, viewLights, litPart } from './tools.js';
import { deployChute, stepChute, aloft, cutAway, lookOf, poseRig, canopyRig, LooseCanopy, MIN_DEPLOY, DRAPE_AHEAD, dropFor, LINKS_AT } from './parachute.js';

// Under a canopy: how far below the eye the risers meet the harness, which is where
// the canopy hangs from, and how much of the walker's swing under it the head keeps
// rather than leveling out.
// Implements: REQ-TOOL-076
const SHOULDERS = 0.15, NOD = 0.45;
// Scratch for hanging the canopy (hangCanopy, rigWorld), and what hang() hands back.
const RIG_AT = new THREE.Vector3(), RIG_TURN = new THREE.Euler(), RIG_QUATERNION = new THREE.Quaternion();
const RIG_SCALE = new THREE.Vector3(1, 1, 1), RIG_MATRIX = new THREE.Matrix4(), RIG_HAND = new THREE.Vector3();
const HANG = { eye: 0, pitch: 0, roll: 0 }, HANG_NONE = Object.freeze({ eye: 0, pitch: 0, roll: 0 });
const RISERS = [], RIG_INVERSE = new THREE.Matrix4();

export const canopy = {
  /**
   * F with the parachute in hand: the pilot chute goes out and the canopy follows it.
   * It has to be thrown from high enough to be worth throwing and with a packed pack;
   * anything else is said rather than done, and the gesture is a pat on the pouch.
   *
   * The canopy opens on the heading the walker is facing, and takes over what they
   * were doing: falling, flying, being pulled along a line.
   *
   * Implements: REQ-TOOL-070, REQ-TOOL-072
   */
  deploy() {
    const p = this.p, tool = this.secondary, chute = this.chute;
    if (aloft(chute)) {
      this.flash('It is open - Space flares it, and putting it away cuts it loose');
      return;
    }
    if (this.dry.has(tool.id) || this.tank(tool) < 1) {
      this.flash(`Still being repacked - ${Math.floor(this.tank(tool) * 100)}% packed`);
      return;
    }
    const clear = p.feet - this.height(p.x, p.z, p.feet);
    if (p.ground || clear < MIN_DEPLOY) {
      this.flash('Too low to open it - it wants a roof or a jet under you');
      return;
    }
    this.cutLine();
    deployChute(chute, { x: p.x, feet: p.feet, z: p.z, vx: this.drift.x, vy: p.vy, vz: this.drift.z, heading: p.yaw });
    this.spend(tool);
    this.fell = null; // from here the canopy decides what the ground costs
    if (this.offhand) this.offhand.userData.throwing = true;
    this.flash('Pilot chute out');
    this.quip('chute');
    this.drawHud();
  },

  /** Empties a pack: a parachute out of its container is a parachute to repack. */
  spend(tool) {
    this.tanks.set(tool.id, 0);
    this.dry.add(tool.id);
  },

  /**
   * One frame under the canopy. The canopy flies the walker and the keys fly the
   * canopy: W and S trim it, A and D (or the arrows) turn it, Space flares it. The
   * mouse still looks about, and the body turns with the canopy, so a turn carries
   * the view round with it the way it would anybody hanging under one.
   *
   * Implements: REQ-TOOL-072, REQ-TOOL-073, REQ-TOOL-074, REQ-TOOL-075
   */
  glide(deltaTime) {
    const k = this.keys, p = this.p, chute = this.chute;
    const forward = (k.has('KeyW') || k.has('ArrowUp') ? 1 : 0) - (k.has('KeyS') || k.has('ArrowDown') ? 1 : 0);
    const turn = (k.has('KeyA') || k.has('ArrowLeft') ? 1 : 0) - (k.has('KeyD') || k.has('ArrowRight') ? 1 : 0);
    // Wherever something else has put the walker since the last frame - a relayout,
    // the edge of the map - is where the canopy is.
    Object.assign(chute, { x: p.x, z: p.z, feet: p.feet, wind: this.airNow });
    const heading = chute.heading;
    const events = stepChute(chute, { forward, turn, flare: k.has('Space') }, deltaTime,
      (x, z, from) => this.height(x, z, from), STEP);
    p.yaw += chute.heading - heading;
    Object.assign(p, { x: chute.x, z: chute.z, feet: chute.feet, vy: chute.vy, ground: false });
    this.confine();
    this.wind.breathe(deltaTime, false);
    this.pace += (0.3 - this.pace) * Math.min(1, deltaTime * 7);
    this.burn(deltaTime, true);
    this.fell = null;
    this.sinking = false;
    for (const e of events) {
      if (e.kind === 'wall') this.struck(e.speed);
      else this.touchdown(e);
    }
    if (this.dying === null) {
      this.bites();
      this.scorches();
    }
    this.health.mend(deltaTime);
  },

  /**
   * A wall, flown into. It costs what arriving at that speed costs, which is nothing
   * at a crawl and a good deal at full flight.
   *
   * Implements: REQ-TOOL-075
   */
  struck(speed) {
    const damage = this.health.touchdown(speed);
    if (!damage) return;
    // In a person's meters a second, the walker being half a unit tall (health.js).
    if (this.health.dead) this.die(`A wall at ${Math.round(speed * 3.5)} meters a second`);
    else {
      this.flash(`That wall cost ${damage} - steer clear of the buildings`);
      this.ouch('wall');
    }
  },

  /**
   * The ground, under a canopy. What it costs is how fast the walker arrives, not how
   * far they came down: the sink and a share of the speed along the ground, which is
   * what a flare is for. A canopy that had not finished opening has done less, and is
   * judged more like the fall it nearly was (Health.touchdown). Water costs nothing,
   * though it is still water.
   *
   * Implements: REQ-TOOL-075, REQ-TOOL-077
   */
  touchdown(e) {
    const p = this.p;
    p.ground = true;
    p.vy = 0;
    this.lay('landed');
    // Down is done with: the harness comes off and the off hand is free, rather than
    // still holding a pack that is lying on the ground being repacked. It repacks as
    // well put away as in hand (burn), and is taken out again for the next jump.
    // Implements: REQ-TOOL-077
    if (this.secondary?.glides) {
      this.secondary = null;
      if (this.active) {
        this.showTool();
        this.drawSlots();
      }
    }
    const wet = this.height(p.x, p.z, p.feet) <= WATER;
    const damage = wet ? 0 : this.health.touchdown(e.speed, e.open, dropFor(e.vertical));
    if (this.health.dead) this.die(`A landing at ${Math.round(e.speed * 3.5)} meters a second`);
    else if (damage) {
      this.flash(`That landing cost ${damage} - flare just before the ground`);
      this.ouch('fall');
    } else {
      this.flash(wet ? 'Down in the water' : 'Down - the parachute is being repacked');
      if (!wet) this.quip('landed');
    }
    this.drawHud();
  },

  /**
   * Lets go of the canopy in the air. It flies off on its own, emptying as it goes,
   * and the walker falls from where they are - which is a fall like any other.
   * Returns whether there was a canopy to let go of.
   *
   * Implements: REQ-TOOL-078
   */
  cutAwayCanopy() {
    if (!aloft(this.chute)) return false;
    this.lay('cutaway');
    cutAway(this.chute);
    this.p.vy = this.chute.vy;
    this.p.ground = false;
    this.fell = this.p.feet;
    this.flash('Cut away - the canopy is gone, and so is what was holding you up');
    this.quip('cutAway');
    this.drawHud();
    return true;
  },

  /**
   * Leaves the canopy where it is, out in the scene: on the ground to be repacked, or
   * cut away and drifting. What is over the walker's head is taken down.
   */
  lay(kind) {
    const chute = this.chute;
    const from = this.rigWorld(new THREE.Matrix4());
    // A canopy on the ground comes down on what is ahead of the walker, where it drapes
    // to: over the edge of a roof onto the street below, but never up a wall.
    const here = this.height(chute.x, chute.z, chute.feet);
    const there = this.height(chute.x - Math.sin(chute.heading) * DRAPE_AHEAD, chute.z - Math.cos(chute.heading) * DRAPE_AHEAD, chute.feet);
    const ground = kind === 'landed' && there <= here + STEP ? there : here;
    const life = kind === 'landed' ? toolFor('parachute').fuel.fills : undefined;
    const velocity = { x: chute.vx, y: chute.vy, z: chute.vz };
    const look = lookOf(chute, performance.now());
    // One that came down before it had finished opening lies there as far open as it got.
    if (kind === 'landed' && chute.touchdown) look.open = chute.touchdown.open;
    this.canopies.push(new LooseCanopy(this.scene, kind, from, look, { ground, velocity, life }));
    if (this.rig?.parent) this.rig.parent.remove(this.rig);
  },

  /**
   * Where the canopy's rig hangs in the world: at the walker's shoulders - seen from
   * behind, at the harness the body wears (body.js risers) - turned to the canopy's
   * heading and tilted by the walker's swing under it (parachute.js), into `out`.
   */
  rigWorld(out) {
    const p = this.p, swing = this.chute.swing;
    RIG_TURN.set(swing.pitch, this.chute.heading, swing.roll, 'YXZ');
    const harness = this.thirdPerson && this.body?.risers(RISERS);
    if (harness) RIG_AT.copy(harness[0]).add(harness[1]).add(harness[2]).add(harness[3]).multiplyScalar(0.25);
    else RIG_AT.set(p.x, p.feet + EYE - SHOULDERS, p.z);
    return out.compose(RIG_AT, RIG_QUATERNION.setFromEuler(RIG_TURN), RIG_SCALE);
  },

  /**
   * The canopy over the walker's head, once a frame after the camera is placed. It is
   * hung from the camera and drawn in the pass that draws what the walker holds, lit
   * by the same lights - it is nearer to them than anything in the street, and a wall
   * drawn through it would be a wall drawn through the lines they are hanging from -
   * and it is given the world's orientation back, so that looking about does not turn
   * the canopy with the eyes. The left brake line ends at the toggle in the left hand.
   *
   * It is drawn whether or not the hands are (H): it is not held.
   *
   * Implements: REQ-TOOL-071, REQ-TOOL-076
   */
  hangCanopy(now) {
    const cam = this.scene.walkCamera;
    if (!aloft(this.chute) || this.arrival) {
      if (this.rig?.parent) this.rig.parent.remove(this.rig);
      if (!this.held && !this.ballView && cam.parent === this.scene.viewScene) this.hideTool();
      return;
    }
    this.rig ||= canopyRig(litPart, (color, options) => new THREE.LineBasicMaterial({ color, ...options }));
    if (cam.parent !== this.scene.viewScene) this.scene.viewScene.add(cam);
    this.lights ||= viewLights();
    if (this.lights.parent !== cam) cam.add(this.lights);
    if (this.rig.parent !== cam) {
      cam.add(this.rig);
      this.rig.matrixAutoUpdate = false;
      this.rig.traverse(o => { o.renderOrder = 9; });
    }
    cam.updateMatrixWorld();
    this.rigWorld(RIG_MATRIX);
    this.rig.matrix.copy(cam.matrixWorld).invert().multiply(RIG_MATRIX);
    this.rig.matrixWorldNeedsUpdate = true;
    // The left toggle in the hand that holds it: the body's, seen from behind.
    const holder = this.bodyHolds ? this.body?.carried.L?.copy : this.offhand;
    const top = holder?.getObjectByName('toggle-top');
    let hand = null;
    RIG_INVERSE.copy(RIG_MATRIX).invert();
    if (top?.parent?.visible) {
      top.updateWorldMatrix(true, false);
      hand = top.getWorldPosition(RIG_HAND).applyMatrix4(RIG_INVERSE);
    }
    poseRig(this.rig, lookOf(this.chute, now), hand);
    this.hangRisers(RIG_INVERSE);
  },

  /**
   * Seen from behind, the risers run from the harness on the body's shoulders up to the
   * links, in place of the ones hung for the eye - which start out to the sides of the
   * head so as to stay out of the view. `inverse` takes the world into the rig.
   */
  hangRisers(inverse) {
    const risers = this.rig.getObjectByName('risers');
    let strapped = this.rig.getObjectByName('harness-risers');
    const harness = this.thirdPerson && risers.visible && this.body?.risers(RISERS);
    // Only the webbing is redrawn: the links and the stowed toggle stay where they are.
    for (const riser of risers.children) if (riser.name === 'riser') riser.visible = !harness;
    if (!harness) {
      if (strapped) strapped.visible = false;
      return;
    }
    if (!strapped) {
      strapped = new THREE.LineSegments(new THREE.BufferGeometry(), new THREE.LineBasicMaterial({ color: '#2b3138' }));
      strapped.geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(LINKS_AT.length * 6), 3));
      strapped.name = 'harness-risers';
      strapped.frustumCulled = false;
      strapped.renderOrder = 9;
      this.rig.add(strapped);
    }
    strapped.visible = true;
    const at = strapped.geometry.getAttribute('position');
    LINKS_AT.forEach((link, i) => {
      const from = RIG_HAND.copy(harness[i]).applyMatrix4(inverse);
      at.setXYZ(i * 2, from.x, from.y, from.z);
      at.setXYZ(i * 2 + 1, link.x, link.y, link.z);
    });
    at.needsUpdate = true;
  },

  /**
   * What hanging under the canopy does to the view: the head swings with the body
   * under it - rolling with the lines in a turn, nodding with a surge or a flare - and
   * dips as the lines take the opening jolt.
   *
   * Implements: REQ-TOOL-076
   */
  hang() {
    const swing = this.chute.swing;
    if (!aloft(this.chute) || reducedMotion()) return HANG_NONE;
    HANG.eye = swing.sag;
    HANG.pitch = swing.pitch * NOD;
    HANG.roll = swing.roll;
    return HANG;
  },

  /** One frame of every canopy left lying about or drifting off. */
  updateCanopies(deltaTime) {
    if (this.canopies.length) this.canopies = this.canopies.filter(c => c.update(deltaTime, this.airNow));
  },

  /** Takes every canopy that was left behind off the map. */
  dropCanopies() {
    for (const canopy of this.canopies) canopy.dispose();
    this.canopies = [];
  },
};
