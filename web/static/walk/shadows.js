// The shadows of what moves in walk mode: the walker, the balls on the courts and the
// bugs. Each frame each of them lays a shadow (shapes.js shadowSpot) on what is under
// it - the walker and a ball on the ground, a roof or a ramp straight below them, a bug
// on whatever it walks, a wall as well as a roof - wider and fainter the higher above
// it they are, so a jump shows how high it went and where it will come down. What
// stands still - a tree, a lamp, a swing - has its shadow with the props (city.js).
//
// Implements: REQ-WALK-064

import * as THREE from '../vendor/three.module.min.js';
import { shadowSpot, shadowMaterial } from '../map/shapes.js';
import { POINT } from './walkbase.js';

// The most shadows drawn in a frame: the walker, a court's ball or two, and the bugs
// nearest them.
const MAX = 160;
// How far from the walker a bug's shadow is drawn; past it the fog has it anyway.
const NEAR = 30;
// Each caster's shadow: how wide across its middle, and how high above the ground it
// is gone.
const WALKER = { r: 0.12, gone: 2.2 };
const BUG = { r: 0.13, gone: 1.4 };
const BALL = { gone: 0.9 };
// A bug stands this far off what it walks (bugs.js moveTo's bob, at rest).
const BUG_STANDS = 0.035;
// Steeper than this, what is under a caster is an edge between two heights, not a
// slope, and the shadow lies flat.
const SLOPE = 1.2;

/**
 * The shadow of something `height` above the ground, `r` across: how wide it is drawn
 * and how dark, 0 to 1, against how dark it is touching the ground - wider and fainter
 * the higher it is, gone at `gone`.
 */
export function shadowOf(height, r, gone) {
  const up = Math.max(0, height) / gone;
  const k = Math.max(0, 1 - up);
  return { size: r * (1 + 0.8 * Math.min(1, up)), dark: k * k * (3 - 2 * k) };
}

export class Shadows {
  constructor(scene) {
    this.scene = scene;
    const material = shadowMaterial(scene.bendable(new THREE.MeshBasicMaterial({ vertexColors: true })));
    // ... each one faded to nothing by its own darkness (aDark), from white.
    const compile = material.onBeforeCompile;
    material.onBeforeCompile = (shader, renderer) => {
      compile.call(material, shader, renderer);
      shader.vertexShader = 'attribute float aDark;\nvarying float vDark;\n'
        + shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\nvDark = aDark;');
      shader.fragmentShader = 'varying float vDark;\n'
        + shader.fragmentShader.replace('#include <color_fragment>', '#include <color_fragment>\ndiffuseColor.rgb = mix(vec3(1.0), diffuseColor.rgb, vDark);');
    };
    material.customProgramCacheKey = () => 'bend-shadow-fading';
    const geometry = shadowSpot();
    this.dark = new THREE.InstancedBufferAttribute(new Float32Array(MAX), 1);
    this.dark.setUsage(THREE.DynamicDrawUsage);
    geometry.setAttribute('aDark', this.dark);
    this.mesh = new THREE.InstancedMesh(geometry, material, MAX);
    this.mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.mesh.frustumCulled = false; // bent in walk mode, and spread over the map
    this.mesh.renderOrder = -1;
    this.mesh.count = 0;
    this.mesh.visible = false;
    scene.scene.add(this.mesh);
    this.n = 0;
  }

  /** Hides them all, out of walk mode. */
  hide() {
    this.mesh.visible = false;
  }

  /**
   * This frame's shadows, for the walker `w` (walk.js Walker): theirs while their body
   * is drawn, the balls', and the bugs' near them.
   */
  update(w) {
    this.n = 0;
    const p = w.p, height = (x, z, from) => w.height(x, z, from, POINT);
    if (w.body?.group.visible && !(w.swim > 0.5) && !w.sinking) {
      // Standing, on what they stand on; in the air, on what is straight under them.
      const ground = p.ground ? p.feet : height(p.x, p.z, p.feet);
      this.lay(p.x, ground, p.z, p.feet - ground, WALKER.r, WALKER.gone, w);
    }
    for (const ball of w.balls?.values() || []) {
      const at = ball.mesh?.position;
      if (!at || !ball.mesh.visible) continue;
      const ground = height(at.x, at.z, at.y);
      this.lay(at.x, ground, at.z, at.y - ball.r - ground, ball.r * 1.1, BALL.gone, w);
    }
    if (w.bugs?.group.visible) {
      for (const bug of w.bugs.bugs) {
        if (bug.caught || bug.take || this.n >= MAX) continue;
        const at = bug.position;
        if (Math.abs(at.x - p.x) > NEAR || Math.abs(at.z - p.z) > NEAR) continue;
        const r = BUG.r * (bug.scale ?? 1);
        if (bug.flying) {
          const ground = height(at.x, at.z, at.y);
          this.lay(at.x, ground, at.z, at.y - ground, r, BUG.gone, w);
        } else {
          // On what it walks: a wall's shadow stands out of the wall.
          const u = bug.up;
          this.place(at.x - u.x * BUG_STANDS, at.y - u.y * BUG_STANDS, at.z - u.z * BUG_STANDS, u, r, 1);
        }
      }
    }
    this.mesh.count = this.n;
    this.mesh.visible = this.n > 0;
    this.mesh.instanceMatrix.needsUpdate = true;
    this.dark.needsUpdate = true;
  }

  /**
   * A shadow at (x, ground, z) of something `above` that, `r` across, gone `gone` up -
   * laid along the slope under it, as on a ramp.
   */
  lay(x, ground, z, above, r, gone, w) {
    const { size, dark } = shadowOf(above, r, gone);
    if (dark <= 0.01) return;
    const d = Math.max(0.05, size * 0.7);
    const dx = (w.height(x + d, z, ground + 0.05, POINT) - w.height(x - d, z, ground + 0.05, POINT)) / (2 * d);
    const dz = (w.height(x, z + d, ground + 0.05, POINT) - w.height(x, z - d, ground + 0.05, POINT)) / (2 * d);
    const flat = Math.abs(dx) > SLOPE || Math.abs(dz) > SLOPE;
    this.place(x, ground, z, flat ? UP : NORMAL.set(-dx, 1, -dz).normalize(), size, dark);
  }

  /** One shadow, at (x, y, z) on a surface facing `normal`, `size` across and `dark`. */
  place(x, y, z, normal, size, dark) {
    if (this.n >= MAX) return;
    TURN.setFromUnitVectors(UP, normal);
    this.mesh.setMatrixAt(this.n, MATRIX.compose(AT.set(x, y, z), TURN, SIZE.set(size, 1, size)));
    this.dark.array[this.n] = dark;
    this.n++;
  }

  dispose() {
    this.mesh.removeFromParent();
    this.mesh.geometry.dispose();
    this.mesh.material.dispose();
  }
}

const UP = new THREE.Vector3(0, 1, 0), NORMAL = new THREE.Vector3();
const AT = new THREE.Vector3(), SIZE = new THREE.Vector3(), TURN = new THREE.Quaternion(), MATRIX = new THREE.Matrix4();
