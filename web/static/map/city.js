// The look of the map, all procedural (no image assets): shader "textures" for the
// boxes, plus props, on the isometric map and in walk mode alike; walk mode adds a sky
// dome and rippling water.
//
// Three styles dress the same geometry (uStyle, set by MapScene.setStyle). A city:
// facades and roofs, a street network on every terrace, grassy shores, trees and
// lamps. A circuit board: chip packages, copper traces down every street, solder mask,
// capacitors and LEDs. A galaxy: crystal spires, glowing conduits, dust and beacons.
// The three share all their geometry and all their measurements - only the paint
// differs - so a style is a look, not a second renderer.
//
// The textures are unlit and antialiased by pixel footprint, so detail fades out
// when zoomed out rather than flickering.
//
// Streets - traces on a board, conduits in a galaxy - are the free space of a terrace
// top: everything not covered by a child
// (building, district, nested terrace, symbol plot). So they connect by
// construction - the gaps between buildings are side streets, the padding along a
// terrace's edge its ring road. The shader measures, per fragment, the distance to
// the nearest obstacles (the terrace's own edges and the footprints listed for its
// cell in a lookup texture, see setRoads): close to one is sidewalk, halfway between
// two facing ones is the center line, and where facing obstacles end is a crossing.
//
// Colors stay data first: facades and roofs are modulated from the box's own
// color (language, size, history, dimming, hover), never replaced. Streets and lawns
// have colors of their own, tinted by how far the box's color is from its kind's
// usual one (uGroundRef, uLandRef): nesting levels, hover and flashes still show.

import * as THREE from '../vendor/three.module.min.js';
import { plants } from './models.js';
import { NOISE_GLSL } from './cityglsl.js';
import { Scatter } from './lod.js';
import { raised } from './buildings.js';

/** Box kinds as the shaders see them (attribute aKind). */
export function kindCode(b) {
  switch (b.kind) {
    case 'land': return 0;
    case 'terrace': return b.node.kind === 'file' ? 6 : 1; // a file's symbol plot is paved, not a block
    case 'district': return 2;
    case 'building': return 3;
    case 'symbol': return 4;
    case 'package': return 5;
  }
  return 3;
}

// ------------------------------------------------------------------ street lookup

// Street shading needs, per fragment, the obstacles near it. A grid over the map
// lists for each cell the ROAD_SLOTS footprints nearest to it (within ROAD_REACH,
// wider than the widest street's half), so the shader never loops over all boxes.
const ROAD_CELL = 0.5, ROAD_REACH = 0.9, ROAD_SLOTS = 8, RECT_TEX_W = 1024;
const MAX_ROAD_CELLS = 1 << 20, MAX_ROAD_TEX = 4096; // memory and texture-size bounds

/** Uniforms the street shader reads; setRoads fills them. */
export function roadUniforms() {
  return {
    uRoadIdx: { value: null }, uRoadRects: { value: null },
    uRoadGrid: { value: new THREE.Vector4() }, uRoadOn: { value: 0 },
    uGroundRef: { value: new THREE.Color(1, 1, 1) }, uLandRef: { value: new THREE.Color(1, 1, 1) },
  };
}

/**
 * Builds the street lookup for a layout: every box but land is an obstacle for the
 * terrace it stands on. A fragment ignores footprints it lies inside (its own
 * terrace and those below it), so one grid serves all terrace levels.
 *
 * Implements: REQ-CITY-016
 */
export function setRoads(u, boxes) {
  for (const k of ['uRoadIdx', 'uRoadRects']) {
    u[k].value?.dispose();
    u[k].value = null;
  }
  u.uRoadOn.value = 0;
  const obs = boxes.filter(b => b.kind !== 'land');
  if (!obs.length) return;
  let minX = Infinity, maxX = -Infinity, minZ = Infinity, maxZ = -Infinity;
  for (const b of obs) {
    minX = Math.min(minX, b.x - b.w / 2); maxX = Math.max(maxX, b.x + b.w / 2);
    minZ = Math.min(minZ, b.z - b.d / 2); maxZ = Math.max(maxZ, b.z + b.d / 2);
  }
  // Huge maps get coarser cells; more obstacles then share a cell's slots.
  const per = Math.min(1 / ROAD_CELL, Math.sqrt(MAX_ROAD_CELLS / Math.max(1, (maxX - minX) * (maxZ - minZ))),
    MAX_ROAD_TEX / 2 / Math.max(1, maxX - minX), MAX_ROAD_TEX / Math.max(1, maxZ - minZ));
  const W = Math.ceil((maxX - minX) * per) + 1, H = Math.ceil((maxZ - minZ) * per) + 1;
  const dist = new Float32Array(W * H * ROAD_SLOTS).fill(Infinity);
  const ids = new Float32Array(W * H * ROAD_SLOTS).fill(-1); // doubles as the texture: 2 RGBA texels per cell
  const rects = new Float32Array(Math.ceil(obs.length / RECT_TEX_W) * RECT_TEX_W * 4);
  const cellOf = (v, min, n) => Math.min(n - 1, Math.max(0, Math.floor((v - min) * per)));

  obs.forEach((b, id) => {
    const x0 = b.x - b.w / 2, x1 = b.x + b.w / 2, z0 = b.z - b.d / 2, z1 = b.z + b.d / 2;
    rects.set([x0, z0, x1, z1], id * 4);
    const i0 = cellOf(x0 - ROAD_REACH, minX, W), i1 = cellOf(x1 + ROAD_REACH, minX, W);
    const j0 = cellOf(z0 - ROAD_REACH, minZ, H), j1 = cellOf(z1 + ROAD_REACH, minZ, H);
    const inL = cellOf(x0, minX, W) + 1, inR = cellOf(x1, minX, W) - 1; // cells wholly inside, by column
    for (let j = j0; j <= j1; j++) {
      const cz0 = minZ + j / per, cz1 = cz0 + 1 / per;
      const rowInside = cz0 >= z0 && cz1 <= z1;
      for (let i = i0; i <= i1; i++) {
        // A cell inside the footprint never shows a surface the footprint borders.
        if (rowInside && i >= inL && i <= inR) { i = inR; continue; }
        const cx0 = minX + i / per, cx1 = cx0 + 1 / per;
        const d = Math.hypot(Math.max(0, x0 - cx1, cx0 - x1), Math.max(0, z0 - cz1, cz0 - z1));
        if (d > ROAD_REACH) continue;
        const base = (j * W + i) * ROAD_SLOTS;
        let worst = base;
        for (let k = base + 1; k < base + ROAD_SLOTS; k++) if (dist[k] > dist[worst]) worst = k;
        if (d < dist[worst]) {
          dist[worst] = d;
          ids[worst] = id;
        }
      }
    }
  });

  const tex = (data, w, h) => {
    const t = new THREE.DataTexture(data, w, h, THREE.RGBAFormat, THREE.FloatType);
    t.minFilter = t.magFilter = THREE.NearestFilter;
    t.generateMipmaps = false;
    t.needsUpdate = true;
    return t;
  };
  u.uRoadIdx.value = tex(ids, W * 2, H);
  u.uRoadRects.value = tex(rects, RECT_TEX_W, rects.length / 4 / RECT_TEX_W);
  u.uRoadGrid.value.set(minX, minZ, per, RECT_TEX_W);
  u.uRoadOn.value = 1;
}

// ------------------------------------------------------------------ sky and water

/**
 * A sky dome drawn behind everything; it follows the camera.
 *
 * Implements: REQ-CITY-030, REQ-MAP-057
 */
export function makeSky(uniforms) {
  const mat = new THREE.ShaderMaterial({
    uniforms: { ...uniforms, uTop: { value: new THREE.Color() }, uHorizon: { value: new THREE.Color() }, uCloud: { value: new THREE.Vector2() } },
    side: THREE.BackSide,
    depthWrite: false,
    vertexShader: `
      varying vec3 vDir;
      void main() {
        vDir = position;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        gl_Position.z = gl_Position.w; // on the far plane
      }`,
    fragmentShader: NOISE_GLSL + `
      uniform vec3 uTop, uHorizon;
      uniform vec2 uCloud;
      uniform float uNight, uTime, uStyle;
      varying vec3 vDir;

      // The plane of the galaxy: a great circle the band of light lies along.
      const vec3 GALACTIC = vec3(0.4, 0.31, -0.86);

      /**
       * One layer of stars. The directions are cut into cells and a cell either holds
       * a star at its center or holds nothing; what was drawn before was the cell
       * itself, which is why the sky was a grid of white squares. Only a few cells in
       * a hundred are lit, so their lattice never reads, and the distance is measured
       * across the line of sight rather than through it, which keeps a star a round
       * dot however the cube grid happens to cross the sphere.
       *
       * color runs from cool blue through white to amber, because a sky of identical
       * white dots reads as dirt on the screen.
       *
       * A star smaller than a pixel would flicker as the view turned, so one that far
       * away is widened to a pixel and dimmed by as much as it was widened: it fades
       * out instead of sparkling. fwidth is taken before anything branches, because a
       * derivative asked for inside a branch is not defined.
       */
      vec3 stars(vec3 d, float scale, float rarity, float size, float glow) {
        float px = length(fwidth(d)) * scale * 0.8;
        vec3 q = d * scale;
        vec3 cell = floor(q);
        float pick = hash13(cell + 5.0);
        if (pick < rarity) return vec3(0.0);
        vec3 off = fract(q) - 0.5;
        off -= d * dot(off, d); // only the part across the view
        float r = length(off);
        // Never wider than a third of a cell: past that the cut at the cell's edge is
        // what shows, which is the grid of squares this was drawn to get rid of. What
        // the widening costs is worked out first, so a capped star still goes out.
        float fade = (size * size) / max(size * size, px * px);
        float w = min(max(size, px), 0.3);
        float core = exp(-(r * r) / (w * w)) * fade;
        float mag = 0.2 + 0.8 * hash13(cell + 3.0);
        // Cooler or warmer than white, either way; a few are decidedly amber.
        float temp = hash13(cell + 29.0);
        vec3 tint = mix(vec3(0.62, 0.74, 1.0), vec3(1.0, 0.84, 0.6), smoothstep(0.35, 0.9, temp));
        tint = mix(vec3(0.95, 0.97, 1.0), tint, 0.8);
        // Slow, per-star, and small: a night sky is not a string of fairy lights.
        float twinkle = 0.9 + 0.1 * sin(uTime * (0.5 + mag) + pick * 39.0);
        return tint * mag * twinkle * (core + glow * exp(-r * r * 70.0) * mag);
      }

      /**
       * Noise over a direction rather than over a plane. Projecting the sky onto one
       * plane pinches everything into a smear at the pole; this takes all three
       * projections and weighs each by how square-on the direction is to it, so the
       * structure is the same size wherever it is looked at. Three octaves, not
       * five: at the size of a nebula the last two are below a pixel.
       */
      float skyFbm(vec3 d, float s, float seed) {
        vec3 w = abs(d);
        w /= w.x + w.y + w.z;
        float v = 0.0;
        vec2 a = d.zy * s + seed, b = d.xz * s + seed + 19.0, c = d.xy * s + seed + 43.0;
        float amp = 0.5;
        for (int i = 0; i < 3; i++) {
          v += amp * (w.x * vnoise(a) + w.y * vnoise(b) + w.z * vnoise(c));
          a = a * 2.03 + 17.1; b = b * 2.03 + 17.1; c = c * 2.03 + 17.1;
          amp *= 0.5;
        }
        return v / 0.875; // the three octaves sum to less than one
      }

      // Deep space: the band of the galaxy with the dark lanes across it, two nebulae
      // drifting behind that, and three layers of stars - dense and faint, sparse and
      // bright, and a few near enough to have a halo around them.
      vec3 space(vec3 d) {
        vec3 col = mix(vec3(0.02, 0.017, 0.05), vec3(0.006, 0.006, 0.022), pow(max(d.y, 0.0), 0.6));

        // The band. Away from its plane there is almost nothing; along it, the light
        // of everything too far off to be a star, broken by the dust that crosses it.
        float band = dot(d, normalize(GALACTIC));
        float along = exp(-band * band * 14.0);
        col += vec3(0.16, 0.15, 0.25) * along * (0.35 + 1.05 * smoothstep(0.3, 0.88, skyFbm(d, 2.6, 0.0)));
        col *= 1.0 - 0.6 * along * smoothstep(0.44, 0.9, skyFbm(d, 5.5, 31.0));

        // Two nebulae, drifting. They are patches of color, not a wash: spread over
        // the whole dome they only lift the black to a flat mauve and take the depth
        // out of everything in front of them.
        //
        // One field of noise serves both, the second reading it upside down: where
        // the purple is thickest the teal is absent, which is what two clouds at
        // different distances look like anyway, and the sky is drawn over every
        // pixel on the screen every frame - it is not the place to ask for noise
        // twice to say the same thing.
        float neb = skyFbm(d, 2.4, uTime * 0.01);
        col += vec3(0.24, 0.07, 0.36) * smoothstep(0.55, 0.88, neb) * (0.5 + 0.5 * neb);
        col += vec3(0.05, 0.19, 0.25) * smoothstep(0.58, 0.82, 1.0 - neb);

        // The scales are chosen so a cell is several pixels across at a normal field
        // of view: finer than that and every star is smaller than a pixel, which is
        // how the dense layer ended up a dim gray haze with wedges cut out of it.
        col += stars(d, 85.0, 0.87, 0.17, 0.0) * (0.5 + 1.0 * along);
        col += stars(d, 38.0, 0.955, 0.13, 0.0) * 1.5;
        col += stars(d, 15.0, 0.982, 0.075, 0.25) * 2.2;
        return col;
      }

      void main() {
        vec3 d = normalize(vDir);
        float up = max(d.y, 0.0);
        if (uStyle > 1.5) {
          gl_FragColor = vec4(space(d), 1.0);
          #include <colorspace_fragment>
          return;
        }
        if (uStyle > 0.5) {
          // A workbench: even, dim light from everywhere, nothing overhead to look at.
          vec3 lab = mix(uHorizon, uTop, pow(up, 0.8)) * (0.95 + 0.05 * vnoise(d.xz * 5.0));
          gl_FragColor = vec4(lab, 1.0);
          #include <colorspace_fragment>
          return;
        }
        // The city's sky. By day it is air lit from one side: deepest straight up,
        // paler and hazier towards the horizon, where the eye looks through more of it,
        // warmer round the sun and a little deeper across the sky from it. Two layers
        // of cloud drift across it with the wind (uCloud, walk.js): cumulus low down,
        // lit on the side facing the sun and gray-blue in their own shadow, with a
        // bright rim where the sun is behind one, and cirrus high up, thin streaks
        // combed out along one way. By night the same clouds are dark against the
        // stars.
        // Implements: REQ-CITY-030
        vec3 sd = normalize(vec3(-0.45, mix(0.42, 0.55, uNight), -0.78));
        float a = dot(d, sd), toSun = max(a, 0.0), day = 1.0 - uNight;
        float shade = 0.0; // how much cloud is in front of the sun's disc
        vec3 col = mix(uHorizon, uTop, pow(up, 0.42));
        col *= 1.0 + 0.07 * a * day;
        col = mix(col, vec3(0.88, 0.92, 0.97), exp(-up * 8.0) * 0.5 * day);
        col += vec3(1.0, 0.78, 0.5) * pow(toSun, 5.0) * (0.16 - 0.08 * up) * day;
        if (d.y > 0.0) {
          // Cumulus, on a layer the view is projected up onto, so they shrink and
          // crowd towards the horizon as clouds overhead do.
          vec2 p = d.xz / (d.y + 0.1) * 1.7 + uCloud;
          vec2 q = p + (vec2(vnoise(p * 0.4), vnoise(p * 0.4 + 5.2)) - 0.5) * 0.45;
          // Billows: the broad shape, its edge broken up finer.
          float dense = fbm(q) + 0.16 * (vnoise(q * 7.0) - 0.5);
          float cover = smoothstep(0.52, 0.6, dense) * smoothstep(0.0, 0.2, d.y);
          // Lit where there is less cloud between it and the sun than in it.
          float behind = fbm(q + normalize(sd.xz) * 0.14);
          float lit = clamp(0.5 + (dense - behind) * 5.5, 0.0, 1.0);
          vec3 cloud = mix(vec3(0.56, 0.62, 0.73), vec3(1.0, 0.985, 0.95), lit);
          cloud *= 0.9 + 0.1 * smoothstep(0.52, 0.75, dense); // thicker is brighter on top
          cloud += vec3(1.0, 0.88, 0.66) * (1.0 - smoothstep(0.52, 0.62, dense)) * pow(toSun, 8.0) * 1.1;
          cloud = mix(cloud, vec3(0.05, 0.06, 0.08) * (0.75 + 0.5 * lit), uNight);
          // Cirrus, higher and so flatter, combed out along one way. The streaks are
          // bent before they are combed, and combed no more than three to one: noise
          // squeezed harder than that shows the square grid it is made on, as straight
          // pale bands across the sky.
          vec2 c = d.xz / (d.y + 0.35) * 0.7 + uCloud * 0.6;
          c += (vec2(vnoise(c * 0.35 + 3.1), vnoise(c * 0.35 + 8.7)) - 0.5) * 1.6;
          c = vec2(c.x * 0.8 - c.y * 0.6, c.x * 0.6 + c.y * 0.8) * vec2(0.9, 2.6);
          float cirrus = smoothstep(0.58, 0.86, fbm(c)) * smoothstep(0.08, 0.45, d.y) * 0.3 * day;
          col = mix(col, vec3(0.97, 0.98, 1.0), cirrus);
          col = mix(col, cloud, cover * mix(0.96, 0.6, uNight));
          shade = cover;
          float star = step(0.998, hash13(floor(d * 420.0))) * uNight * (1.0 - cover) * smoothstep(0.04, 0.3, d.y);
          col += star * (0.4 + 0.6 * hash13(floor(d * 420.0) + 7.0));
        }
        vec3 light = mix(vec3(1.0, 0.92, 0.75), vec3(0.75, 0.78, 0.85), uNight);
        // The disc, dimmed behind a cloud; its glow comes through thin cloud as a
        // brighter patch of it.
        col += light * (smoothstep(0.9986, 0.9991, a) * (1.0 - 0.8 * shade)
          + pow(toSun, 48.0) * mix(0.4, 0.06, uNight) + pow(toSun, 600.0) * 0.6 * day);
        gl_FragColor = vec4(col, 1.0);
        #include <colorspace_fragment>
      }`,
  });
  const sky = new THREE.Mesh(new THREE.SphereGeometry(10, 32, 16), mat);
  sky.frustumCulled = false;
  sky.renderOrder = -10;
  return sky;
}

/**
 * Water with slow ripples: the planet's surface in walk mode, the sea around the
 * isometric map. Ripples fade to their mean where a pixel covers several of them.
 *
 * Implements: REQ-CITY-003, REQ-CITY-025, REQ-CITY-030, REQ-MAP-057
 */
export function waterMaterial(uniforms) {
  const mat = new THREE.MeshBasicMaterial();
  mat.onBeforeCompile = shader => {
    Object.assign(shader.uniforms, uniforms);
    shader.vertexShader = 'varying vec3 vW;\n' + shader.vertexShader.replace('#include <project_vertex>',
      '#include <project_vertex>\nvW = (modelMatrix * vec4(position, 1.0)).xyz;');
    shader.fragmentShader = NOISE_GLSL + `
      uniform float uTime, uNight, uStyle;
      varying vec3 vW;

      const float BUS = 1.6;    // how far apart the backplane's tracks run
      const float TRACK = 0.05; // how wide one is, in lanes
      const float RUN = 7.0;    // and how far apart the pulses on one of them are
      const float BALL = 0.17;  // how big the head of a pulse is, in map units
      const float TAIL = 2.1;   // and how far its wake reaches behind it
      const float FLOW = 0.16;  // runs per second: about one unit and a bit a second
      const vec3 CURRENT = vec3(0.35, 0.78, 1.0);

      /**
       * The light one lane is carrying where this pixel is: a ball of it at the head
       * of a pulse, and the tail the ball drags behind it.
       *
       * It takes both where the pixel is down the lane and how far it is off the
       * lane's center line, because a head that only knows the first is a band of
       * even brightness the width of the track - a straight edge crossing it - and
       * what a current wants to look like is a ball of light with a wake.
       *
       * The lanes alternate direction and each starts at a phase of its own, so the
       * plane reads as traffic rather than as one pattern marching in step.
       *
       * It fades out once a pixel covers a good part of the run between two pulses:
       * a train of comets seen from far enough away is a flicker, and the map view
       * is often seen from exactly that far away.
       */
      float charge(float along, float across, float lane, float w) {
        float dir = mod(lane, 2.0) * 2.0 - 1.0;
        float back = fract(along / RUN + dir * uTime * FLOW + hash12(vec2(lane, 7.0))) * RUN;
        float off = across * BUS; // map units, like everything else here

        // The ball. Its distance from the head is measured along the lane and across
        // it at once, so what falls off is a circle rather than a pair of edges, and
        // it is wider than the track it runs on, so it bulges off either side. The
        // second, broader term is the light it throws around itself.
        float near = min(back, RUN - back);
        float d2 = near * near + off * off;
        float ball = exp(-d2 / (BALL * BALL)) + 0.35 * exp(-d2 / (5.0 * BALL * BALL));

        // The tail, behind the head only, narrowing as it falls away: a tail of one
        // width is a bar, and only one that comes to a point reads as a wake.
        float t = clamp(back / TAIL, 0.0, 1.0);
        float wide = BALL * mix(0.85, 0.12, t);
        float tail = exp(-off * off / (wide * wide)) * (1.0 - t) * (1.0 - t);

        return (ball + 0.8 * tail) * (1.0 - smoothstep(0.03, 0.14, w));
      }

      /**
       * How bright a star is at this moment, between a half and one. Each has its own
       * phase, taken from the cell it sits in, and its own rate from the layer it
       * belongs to - a field that all pulsed together would read as the screen
       * flickering rather than as stars.
       */
      float breathe(vec2 q, float rate) {
        return 0.75 + 0.25 * sin(uTime * rate + hash12(floor(q)) * 43.0);
      }
    ` + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      if (uStyle > 1.5) {
        // Between the platforms there is no sea, only more of the same sky seen the
        // other way. Nothing here is still: the platforms are adrift, and what is
        // behind them moves.
        //
        // Depth is the whole of it. Three layers travel at three speeds and in three
        // directions - the far cloud barely at all, the veil in front of it faster
        // and across it, the stars faster still - which is why it reads as something
        // deep rather than as one sheet of noise sliding sideways. The speeds are in
        // the noise's own coordinates, so the far cloud crosses about a fifth of a
        // map unit a second: a drift, not an animation.
        vec2 far = vW.xz * 0.055 + vec2(uTime * 0.007, uTime * -0.004);
        vec2 veil = vW.zx * 0.1 + vec2(uTime * -0.021, uTime * 0.014);
        float deep = fbm(far);
        diffuseColor.rgb *= 0.42 + 0.42 * vnoise(vW.xz * 0.12 + vec2(uTime * 0.01, 0.0));
        diffuseColor.rgb += vec3(0.13, 0.04, 0.22) * smoothstep(0.52, 0.9, deep) * (0.4 + 0.6 * deep);
        diffuseColor.rgb += vec3(0.02, 0.08, 0.15) * smoothstep(0.62, 0.95, fbm(veil + 23.0));
        // Through the dust, a slow curtain of light: the one thing here bright enough
        // to be seen moving, so it is what makes the rest of it read as moving too.
        // Broad and faint - a sheet drawn across the whole of it, not a cloud, which
        // is the difference between a nebula and mold on the screen.
        float curtain = fbm(vW.xz * 0.045 + vec2(uTime * -0.012, uTime * 0.007) + 61.0);
        diffuseColor.rgb += vec3(0.05, 0.09, 0.19) * smoothstep(0.58, 0.96, curtain);

        // The stars, drifting with the layer they belong to and each breathing at its
        // own rate. One field at one brightness is a speckle; these have a distance.
        vec2 s0 = vW.xz * 34.0 + vec2(uTime * 0.09, uTime * -0.05);
        vec2 s1 = vW.xz * 11.0 + vec2(uTime * -0.14, uTime * 0.08);
        vec2 s2 = vW.zx * 4.5 + vec2(uTime * 0.11, uTime * -0.16) + 7.0;
        diffuseColor.rgb += vec3(0.72, 0.78, 1.0) * starDot(s0, 0.955, 0.16) * 0.8 * breathe(s0, 1.0);
        diffuseColor.rgb += vec3(0.86, 0.9, 1.0) * starDot(s1, 0.985, 0.13) * 1.6 * breathe(s1, 0.7);
        diffuseColor.rgb += vec3(1.0, 0.86, 0.66) * starDot(s2, 0.992, 0.1) * 2.0 * breathe(s2, 0.45);
      } else if (uStyle > 0.5) {
        // Off the edge of the board is the backplane it is plugged into, and it is
        // live. This is the circuit style's water: the place the map will not let
        // you walk. A flat gray bench said nothing about that - it read as a floor -
        // and a bus with charge running down it says it without a word.
        //
        // The plane is dark and brushed, with a copper track down the middle of
        // every lane in both directions, and charge traveling along them: lanes run
        // in alternating directions and start at their own phase, so what is seen is
        // traffic rather than a pattern marching in step.
        vec2 t = vW.xz / BUS;
        vec2 tw = fwidth(t) + 1e-4;
        vec2 mid = abs(fract(t) - 0.5); // how far into a lane, in lanes
        float brush = vnoise(vec2(vW.x * 160.0, vW.z * 2.5)) + 0.5 * vnoise(vec2(vW.x * 420.0, vW.z * 1.3));
        brush = mix(brush, 0.75, smoothstep(0.35, 1.2, fwidth(vW.x * 160.0))); // to its mean, minified
        diffuseColor.rgb *= 0.42 + 0.12 * (brush - 0.75) + 0.08 * vnoise(vW.xz * 8.0);

        // The tracks: a hair of copper down each lane. They are left to soften into
        // the plane as the map is pulled away rather than held to a pixel - a grid
        // of lines finer than the screen can draw is a brown haze, and the pulses
        // are what has to carry this from a distance, not the lines they run on.
        float onX = 1.0 - smoothstep(TRACK - tw.y, TRACK + tw.y, mid.y);
        float onZ = 1.0 - smoothstep(TRACK - tw.x, TRACK + tw.x, mid.x);
        float track = max(onX, onZ) * (1.0 - smoothstep(0.09, 0.3, max(tw.x, tw.y)));
        diffuseColor.rgb = mix(diffuseColor.rgb, vec3(0.3, 0.17, 0.06) * (0.8 + 0.4 * brush), track);

        // The charge. Each lane carries a train of balls of light, running one way
        // on the even lanes and the other on the odd ones, each dragging a wake
        // behind it - which is what says at a glance which way the current runs.
        // The light is its own shape rather than something painted onto the track,
        // so it spills off either side the way light does.
        diffuseColor.rgb += CURRENT * (charge(vW.x, mid.y, floor(t.y), tw.y)
                                     + charge(vW.z, mid.x, floor(t.x), tw.x));
      } else {
        // Open water: a long swell with wind chop riding on it, the crests catching
        // the sky and a little of it showing through where the water is thin. Two
        // octaves traveling at different speeds and angles is what stops it reading
        // as one sheet of noise sliding sideways.
        vec2 w = vW.xz;
        float swell = vnoise(w * 0.7 + vec2(uTime * 0.09, uTime * 0.05))
          + 0.6 * vnoise(w * 1.6 + vec2(uTime * 0.31, -uTime * 0.14));
        float chop = vnoise(w * 4.5 - vec2(uTime * 0.28, uTime * 0.36))
          + 0.5 * vnoise(w * 9.5 + vec2(-uTime * 0.5, uTime * 0.22));
        vec2 fw = fwidth(w * 9.5);
        // A pixel covering many ripples sees their mean; the swell survives longer
        // than the chop does, which is what distance does to water.
        chop = mix(chop / 1.5, 0.5, smoothstep(0.25, 1.0, max(fw.x, fw.y)));
        swell = mix(swell / 1.6, 0.5, smoothstep(0.35, 1.4, max(fw.x, fw.y) * 0.2));
        float r = mix(swell, chop, 0.45);
        diffuseColor.rgb *= 0.76 + 0.46 * r;
        // The crests break rather than glow: a fine sparkle that lives only on the
        // top of a wave. A broad white wash over the peaks, which is what was here,
        // reads as fog lying on the water.
        float sparkle = vnoise(w * 24.0 - vec2(uTime * 0.7, uTime * 0.45));
        sparkle = mix(sparkle, 0.5, smoothstep(0.3, 1.0, fwidth(w.x * 24.0)));
        diffuseColor.rgb += smoothstep(0.64, 0.86, r) * smoothstep(0.54, 0.86, sparkle)
          * mix(0.34, 0.1, uNight);
      }`);
  };
  mat.customProgramCacheKey = () => 'water';
  return mat;
}

// ------------------------------------------------------------------ props

// ------------------------------------------------------------------ blocks and ramps

/** Terraces (city blocks, not a file's symbol plot) with the boxes standing on them. */
function blocks(boxes) {
  const byNode = new Map();
  for (const b of boxes) if (b.kind === 'terrace' && b.node.kind !== 'file') byNode.set(b.node.id, b);
  const kids = new Map([...byNode.values()].map(t => [t, []]));
  for (const b of boxes) {
    if (b.kind === 'land') continue;
    const t = b.node.parentNode && byNode.get(b.node.parentNode.id);
    if (t) kids.get(t).push(b);
  }
  return kids;
}

// A ramp runs along one side of a nested terrace, in the street beside it, from the
// street's level at one corner up to the terrace's top: RAMP_W wide, at most RAMP_MAX
// long, the last RAMP_LANDING of it level. From the landing a driveway, DRIVE deep,
// crosses the terrace's sidewalk into its ring road; at the foot an apron, APRON
// long, replaces the street's curb. The far end leaves room for the stairs (the
// stairs shader).
// Implements: REQ-CITY-017, REQ-CITY-020
const RAMP_W = 0.4, RAMP_MAX = 2.4, RAMP_MIN_SIDE = 1.7, RAMP_START = 0.1, RAMP_CLEAR = 0.1;
const RAMP_LANDING = 0.35, DRIVE = 0.14, APRON = RAMP_START + 0.07;
// A flight of stairs between two plazas: STAIR_LEN long with a STAIR_LANDING at the
// top, STAIR_W wide, each step STEP_RISE high at most. A side shorter than
// STAIR_MIN_SIDE has no room for one.
const STAIR_LEN = 0.8, STAIR_LANDING = 0.12, STAIR_W = 0.24, STEP_RISE = 0.035, STAIR_MIN_SIDE = 1.2;

const rampCache = new WeakMap();

/**
 * The ramps of a layout: for every nested terrace, one on the side with the most room
 * beside it. Each ramp: {origin: [x, z] (low end, at the wall), u: direction up the
 * ramp, n: away from the wall, len, rise (the sloped part's length), y0, y1 (low and
 * high surface), x0, z0, x1, z1 (footprint), drive (the driveway's footprint on the
 * terrace)}. Cached per boxes array: scene and walker share it.
 *
 * Implements: REQ-CITY-017, REQ-CITY-020, REQ-CITY-021
 */
export function rampsFor(boxes) {
  if (rampCache.has(boxes)) return rampCache.get(boxes);
  const ramps = [];
  for (const [t, kids] of blocks(boxes)) {
    for (const c of kids) {
      if (c.kind !== 'terrace' || c.node.kind === 'file') continue;
      // Up to a road, a ramp a car can drive; up to a plaza (buildings.js raised),
      // where nothing drives - or to a block with no side long enough for a ramp - a
      // flight of stairs: shorter, steeper, and no driveway.
      // Implements: REQ-CITY-037
      const best = (!raised(c) && wayUp(t, c, kids, false)) || wayUp(t, c, kids, true);
      if (best) ramps.push(best);
    }
  }
  ramps.push(...approaches(boxes));
  rampCache.set(boxes, ramps);
  return ramps;
}

// The way up to terrace c from the block t it stands on, on the side of c with the
// most room beside it: a ramp, or with stairs a flight; null when no side has room.
function wayUp(t, c, kids, stairs) {
  const tx0 = t.x - t.w / 2, tx1 = t.x + t.w / 2, tz0 = t.z - t.d / 2, tz1 = t.z + t.d / 2;
  let best = null;
  for (const side of sides(c)) {
    if (side.len < (stairs ? STAIR_MIN_SIDE : RAMP_MIN_SIDE)) continue;
    const rampLength = stairs ? STAIR_LEN : Math.min(RAMP_MAX, side.len - 0.8);
    const width = stairs ? STAIR_W : RAMP_W;
    const a = -side.len / 2 + RAMP_START;
    const origin = [side.mid[0] + side.u[0] * a, side.mid[1] + side.u[1] * a];
    const far = [origin[0] + side.u[0] * rampLength + side.n[0] * width, origin[1] + side.u[1] * rampLength + side.n[1] * width];
    const r = {
      x0: Math.min(origin[0], far[0]), x1: Math.max(origin[0], far[0]),
      z0: Math.min(origin[1], far[1]), z1: Math.max(origin[1], far[1]),
    };
    let clear = Math.min(r.x0 - tx0, tx1 - r.x1, r.z0 - tz0, tz1 - r.z1);
    for (const k of kids) if (k !== c) clear = Math.min(clear, rectDist(r, k));
    if (clear >= RAMP_CLEAR && (!best || clear > best.clear)) {
      best = {
        ...r, clear, origin, u: side.u, n: side.n, len: rampLength, width, stairs,
        rise: rampLength - (stairs ? STAIR_LANDING : RAMP_LANDING), y0: t.y + t.h, y1: c.y + c.h, drive: null,
      };
      if (!stairs) {
        const d0 = [origin[0] + side.u[0] * best.rise, origin[1] + side.u[1] * best.rise];
        const d1 = [origin[0] + side.u[0] * rampLength - side.n[0] * DRIVE, origin[1] + side.u[1] * rampLength - side.n[1] * DRIVE];
        best.drive = { x0: Math.min(d0[0], d1[0]), x1: Math.max(d0[0], d1[0]), z0: Math.min(d0[1], d1[1]), z1: Math.max(d0[1], d1[1]) };
      }
    }
  }
  return best;
}

// Where a bridge comes ashore, a ramp as wide as its deck carries the road on, straight
// up the shore, to the ring road of the terrace standing there: the bridge and the
// streets are one road. A bridge that lands off the terrace's side keeps to the shore.
// Implements: REQ-CITY-038
function approaches(boxes) {
  const terraces = new Map();
  for (const b of boxes) if (b.kind === 'terrace') terraces.set(b.node, b);
  const ways = [];
  for (const bridge of bridgesFor(boxes)) {
    const x = bridge.axis === 'x';
    for (const end of [bridge.from, bridge.to]) {
      const shore = [bridge.a, bridge.b].find(l => {
        const r = rect(l);
        return x ? end >= r.x0 && end <= r.x1 : end >= r.z0 && end <= r.z1;
      });
      const t = shore && terraces.get(shore.node);
      if (!t) continue;
      const tr = rect(t);
      const [low, high] = x ? [tr.z0, tr.z1] : [tr.x0, tr.x1];
      if (bridge.across - DECK_W / 2 < low + RAMP_CLEAR || bridge.across + DECK_W / 2 > high - RAMP_CLEAR) continue;
      const toward = end === bridge.from ? -1 : 1; // the shore lies behind the deck's end
      const wall = x ? (toward < 0 ? tr.x1 : tr.x0) : (toward < 0 ? tr.z1 : tr.z0);
      const len = (wall - end) * toward;
      if (len < 0.3) continue;
      const u = x ? [toward, 0] : [0, toward], n = x ? [0, 1] : [1, 0];
      const origin = x ? [end, bridge.across - DECK_W / 2] : [bridge.across - DECK_W / 2, end];
      const footprint = (s0, s1) => {
        const a = [origin[0] + u[0] * s0, origin[1] + u[1] * s0];
        const b = [origin[0] + u[0] * s1 + n[0] * DECK_W, origin[1] + u[1] * s1 + n[1] * DECK_W];
        return { x0: Math.min(a[0], b[0]), x1: Math.max(a[0], b[0]), z0: Math.min(a[1], b[1]), z1: Math.max(a[1], b[1]) };
      };
      ways.push({
        ...footprint(0, len), origin, u, n, len, rise: len, width: DECK_W, stairs: false, approach: true,
        y0: shore.y + shore.h, y1: t.y + t.h, drive: footprint(len, len + DRIVE),
      });
    }
  }
  return ways;
}

/**
 * The ramp surface's height at (x, z), or -Infinity off the ramp.
 *
 * Implements: REQ-CITY-018, REQ-WALK-035
 */
export function rampHeight(r, x, z) {
  if (x < r.x0 || x > r.x1 || z < r.z0 || z > r.z1) return -Infinity;
  const s = ((x - r.origin[0]) * r.u[0] + (z - r.origin[1]) * r.u[1]) / r.rise;
  return r.y0 + (r.y1 - r.y0) * Math.min(1, Math.max(0, s));
}

// A box's four sides: outward normal n, direction u of increasing position along the
// face as the shaders measure it (cityColor's u), the side's center and length.
function sides(b) {
  const hw = b.w / 2, hd = b.d / 2;
  return [
    { n: [1, 0], u: [0, 1], mid: [b.x + hw, b.z], len: b.d },
    { n: [-1, 0], u: [0, -1], mid: [b.x - hw, b.z], len: b.d },
    { n: [0, 1], u: [-1, 0], mid: [b.x, b.z + hd], len: b.w },
    { n: [0, -1], u: [1, 0], mid: [b.x, b.z - hd], len: b.w },
  ];
}

// Distance between two footprints ({x0, z0, x1, z1} or a box), 0 when they overlap.
function rectDist(a, b) {
  const bx0 = b.x0 ?? b.x - b.w / 2, bx1 = b.x1 ?? b.x + b.w / 2, bz0 = b.z0 ?? b.z - b.d / 2, bz1 = b.z1 ?? b.z + b.d / 2;
  return Math.hypot(Math.max(0, a.x0 - bx1, bx0 - a.x1), Math.max(0, a.z0 - bz1, bz0 - a.z1));
}

// ------------------------------------------------------------------ bridges

// Islands are separated from the mainland by water, which a walker cannot cross, so
// every island is linked by a bridge: a deck DECK_W wide, arched by DECK_RISE, with
// railings and piers. The links form a spanning tree rooted at the mainland, each
// island joined to the nearest shore already reachable, so every island can be walked
// to. Shores (the land boxes) all have their tops at the same height.
const DECK_W = 0.8, DECK_RISE = 0.25, DECK_T = 0.06, PIER_EVERY = 1.8, DECK_OVERLAP = 0.25;
/** How high a bridge's railings stand above its deck. */
export const RAIL_H = 0.14;

const bridgeCache = new WeakMap();

/**
 * The bridges of a layout: {a, b: the two shores, axis: 'x'|'z' (the span's
 * direction), across: the deck's center on the other axis, from, to: the span's ends
 * along the axis, y: the shores' level}. Cached per boxes array, like rampsFor.
 *
 * Implements: REQ-CITY-021, REQ-CITY-026
 */
export function bridgesFor(boxes) {
  if (bridgeCache.has(boxes)) return bridgeCache.get(boxes);
  const lands = boxes.filter(b => b.kind === 'land');
  const bridges = [];
  if (lands.length > 1) {
    // The mainland is the largest shore; islands join the nearest shore already linked.
    const linked = [lands.reduce((a, b) => (a.w * a.d >= b.w * b.d ? a : b))];
    const rest = lands.filter(l => l !== linked[0]);
    while (rest.length) {
      let best = null;
      for (const a of linked) {
        for (const b of rest) {
          const d = rectDist(rect(a), b);
          if (!best || d < best.d) best = { a, b, d };
        }
      }
      const span = bridgeBetween(best.a, best.b);
      if (span) bridges.push(span);
      linked.push(best.b);
      rest.splice(rest.indexOf(best.b), 1);
    }
  }
  bridgeCache.set(boxes, bridges);
  return bridges;
}

/** A bridge deck's footprint, for indexing it like a box. */
export function bridgeBounds(r) {
  const half = DECK_W / 2;
  return r.axis === 'x'
    ? { x0: r.from, x1: r.to, z0: r.across - half, z1: r.across + half }
    : { x0: r.across - half, x1: r.across + half, z0: r.from, z1: r.to };
}

/**
 * The deck's height at (x, z), or -Infinity beside the bridge.
 *
 * `inset` narrows the deck on both sides by that much: what a body of that radius
 * may stand on, rather than what is drawn. A bridge has railings, and a walker whose
 * middle is over the deck's own edge is a walker hanging over the water - so the
 * walker asks with their own radius and everything else asks for the deck itself.
 *
 * Implements: REQ-CITY-027, REQ-CITY-028, REQ-WALK-035
 */
export function bridgeHeight(r, x, z, inset = 0) {
  const along = r.axis === 'x' ? x : z, across = r.axis === 'x' ? z : x;
  const half = DECK_W / 2 - inset;
  if (half <= 0 || along < r.from || along > r.to || Math.abs(across - r.across) > half) return -Infinity;
  const t = (along - r.from) / Math.max(1e-6, r.to - r.from);
  return r.y + DECK_RISE * Math.sin(Math.PI * t);
}

// The span between two shores: across the smaller gap, centered on the stretch where
// they face each other (or on the nearer end when they only overlap diagonally).
function bridgeBetween(a, b) {
  const ra = rect(a), rb = rect(b);
  const gapX = Math.max(rb.x0 - ra.x1, ra.x0 - rb.x1), gapZ = Math.max(rb.z0 - ra.z1, ra.z0 - rb.z1);
  const axis = gapX > gapZ ? 'x' : 'z';
  const [g0, g1] = axis === 'x' ? [ra.x1, rb.x0] : [ra.z1, rb.z0];
  const forward = axis === 'x' ? ra.x1 <= rb.x0 : ra.z1 <= rb.z0;
  const from = forward ? g0 : (axis === 'x' ? rb.x1 : rb.z1);
  const to = forward ? g1 : (axis === 'x' ? ra.x0 : ra.z0);
  if (to - from < 0.2) return null; // touching shores need no bridge
  const [low, high] = axis === 'x'
    ? [Math.max(ra.z0, rb.z0), Math.min(ra.z1, rb.z1)]
    : [Math.max(ra.x0, rb.x0), Math.min(ra.x1, rb.x1)];
  const across = low < high
    ? (low + high) / 2
    : clampTo(axis === 'x' ? [ra.z0, ra.z1] : [ra.x0, ra.x1], axis === 'x' ? (rb.z0 + rb.z1) / 2 : (rb.x0 + rb.x1) / 2);
  return { a, b, axis, across, from: from - DECK_OVERLAP, to: to + DECK_OVERLAP, y: a.y + a.h };
}

const rect = b => ({ x0: b.x - b.w / 2, x1: b.x + b.w / 2, z0: b.z - b.d / 2, z1: b.z + b.d / 2 });
const clampTo = ([low, high], v) => Math.min(high - DECK_W, Math.max(low + DECK_W, v));

// Bridge geometry: the arched deck (split along its length so it bends with the
// planet), its sides and railings, and a pier every PIER_EVERY down to the water.
// aRamp, as for ramps: across and along in world units, 3 on the deck (a road with a
// center line), 0 on everything else.
// Implements: REQ-CITY-027
function* bridgeGeometry(bridges) {
  const positions = [], ramp = [], shade = [], index = [];
  const quad = (a, b, c, d, ra, rb, rc, rd, k) => {
    const i = positions.length / 3;
    positions.push(...a, ...b, ...c, ...d);
    ramp.push(...ra, ...rb, ...rc, ...rd);
    for (let j = 0; j < 4; j++) shade.push(k, k, k);
    index.push(i, i + 1, i + 2, i, i + 2, i + 3);
  };
  const box = (cx, cz, y0, y1, w, d, k) => {
    for (const [sx, sz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
      const hw = w / 2, hd = d / 2;
      const p = (dx, dz, y) => [cx + dx, y, cz + dz];
      const [ax, az, bx, bz] = sx ? [sx * hw, -hd, sx * hw, hd] : [-hw, sz * hd, hw, sz * hd];
      quad(p(ax, az, y0), p(bx, bz, y0), p(bx, bz, y1), p(ax, az, y1),
        [0, 0, 0], [w + d, 0, 0], [w + d, y1 - y0, 0], [0, y1 - y0, 0], k);
    }
  };
  for (const r of bridges) {
    const spanLength = r.to - r.from;
    const at = (t, off, dy) => {
      const along = r.from + spanLength * t, y = r.y + DECK_RISE * Math.sin(Math.PI * t) + dy;
      return r.axis === 'x' ? [along, y, r.across + off] : [r.across + off, y, along];
    };
    const n = Math.max(3, Math.ceil(spanLength / 0.5));
    for (let i = 0; i < n; i++) {
      const t0 = i / n, t1 = (i + 1) / n, a0 = spanLength * t0, a1 = spanLength * t1;
      const hw = DECK_W / 2;
      quad(at(t0, -hw, 0), at(t1, -hw, 0), at(t1, hw, 0), at(t0, hw, 0),
        [0, a0, 3], [0, a1, 3], [DECK_W, a1, 3], [DECK_W, a0, 3], 1);          // deck
      for (const side of [-1, 1]) {
        quad(at(t0, side * hw, -DECK_T), at(t1, side * hw, -DECK_T), at(t1, side * hw, 0), at(t0, side * hw, 0),
          [a0, 0, 0], [a1, 0, 0], [a1, DECK_T, 0], [a0, DECK_T, 0], 0.62);     // the deck's edge
        quad(at(t0, side * hw, 0), at(t1, side * hw, 0), at(t1, side * hw, RAIL_H), at(t0, side * hw, RAIL_H),
          [a0, 0, 0], [a1, 0, 0], [a1, RAIL_H, 0], [a0, RAIL_H, 0], 0.9);      // railing
      }
    }
    for (let t = PIER_EVERY / 2; t < spanLength; t += PIER_EVERY) {
      const [x, y, z] = at(t / spanLength, 0, -DECK_T);
      box(x, z, r.y - 0.45, y, 0.18, 0.18, 0.7);
    }
    yield;
  }
  yield;
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  yield;
  geo.setAttribute('aRamp', new THREE.Float32BufferAttribute(ramp, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(shade, 3));
  yield;
  geo.setIndex(index);
  return geo;
}

// Ramp geometry: the roadway (split along its length so it bends with the planet),
// its outer wall with a parapet, the wall and barrier at its high end, the driveway
// onto the terrace and the apron at its foot. aRamp: across and along the roadway in
// world units, and 1 on the sloped roadway (markings), 2 on plain asphalt, 0 on walls.
// Implements: REQ-CITY-019, REQ-CITY-020
function* rampGeometry(ramps) {
  const positions = [], ramp = [], shade = [], index = [];
  const quad = (a, b, c, d, ra, rb, rc, rd, k) => {
    const i = positions.length / 3;
    positions.push(...a, ...b, ...c, ...d);
    ramp.push(...ra, ...rb, ...rc, ...rd);
    for (let j = 0; j < 4; j++) shade.push(k, k, k);
    index.push(i, i + 1, i + 2, i, i + 2, i + 3);
  };
  const LIFT = 0.004, PARAPET = 0.035;
  for (const r of ramps) {
    const width = r.width;
    const at = (s, w, y) => [r.origin[0] + r.u[0] * s + r.n[0] * w, y, r.origin[1] + r.u[1] * s + r.n[1] * w];
    if (r.stairs) {
      // Implements: REQ-CITY-037
      flight(r, at, quad, PARAPET);
      yield;
      continue;
    }
    const y = s => r.y0 + (r.y1 - r.y0) * Math.min(1, s / r.rise) + LIFT;
    if (r.approach) {
      approach(r, at, y, quad, PARAPET);
      yield;
      continue;
    }
    const n = Math.max(2, Math.ceil(r.rise / 0.25));
    const cuts = [...Array.from({ length: n + 1 }, (_, i) => r.rise * i / n), r.len];
    for (let i = 0; i + 1 < cuts.length; i++) {
      const s0 = cuts[i], s1 = cuts[i + 1], k = s0 < r.rise ? 1 : 2;
      quad(at(s0, 0, y(s0)), at(s1, 0, y(s1)), at(s1, width, y(s1)), at(s0, width, y(s0)),
        [0, s0, k], [0, s1, k], [width, s1, k], [width, s0, k], 1);
      // Outer wall, up to the parapet's top, and the parapet's top.
      quad(at(s0, width, r.y0), at(s1, width, r.y0), at(s1, width, y(s1) + PARAPET), at(s0, width, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, y(s1) + PARAPET - r.y0, 0], [s0, y(s0) + PARAPET - r.y0, 0], 0.68);
      quad(at(s0, width - 0.02, y(s0) + PARAPET), at(s1, width - 0.02, y(s1) + PARAPET), at(s1, width, y(s1) + PARAPET), at(s0, width, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, 0.02, 0], [s0, 0.02, 0], 0.95);
      quad(at(s0, width - 0.02, y(s0)), at(s1, width - 0.02, y(s1)), at(s1, width - 0.02, y(s1) + PARAPET), at(s0, width - 0.02, y(s0) + PARAPET),
        [s0, 0, 0], [s1, 0, 0], [s1, PARAPET, 0], [s0, PARAPET, 0], 0.8);
    }
    // The high end: its wall down to the street, and a barrier: traffic turns onto
    // the terrace here.
    quad(at(r.len, 0, r.y0), at(r.len, width, r.y0), at(r.len, width, y(r.len) + PARAPET), at(r.len, 0, y(r.len) + PARAPET),
      [0, 0, 0], [width, 0, 0], [width, r.y1 - r.y0 + PARAPET, 0], [0, r.y1 - r.y0 + PARAPET, 0], 0.75);
    // Driveway across the terrace's sidewalk, and the apron over the street's curb.
    quad(at(r.rise, -DRIVE, y(r.len)), at(r.len, -DRIVE, y(r.len)), at(r.len, 0, y(r.len)), at(r.rise, 0, y(r.len)),
      [-DRIVE, r.rise, 2], [-DRIVE, r.len, 2], [0, r.len, 2], [0, r.rise, 2], 1);
    quad(at(-APRON, 0, r.y0 + LIFT), at(0, 0, r.y0 + LIFT), at(0, width, r.y0 + LIFT), at(-APRON, width, r.y0 + LIFT),
      [0, -APRON, 2], [0, 0, 2], [width, 0, 2], [width, -APRON, 2], 1);
    yield;
  }
  yield;
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  yield;
  geo.setAttribute('aRamp', new THREE.Float32BufferAttribute(ramp, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(shade, 3));
  yield;
  geo.setIndex(index);
  return geo;
}

// A bridge's approach: the deck's road carried on up the shore (aRamp 3, the deck's
// markings), a wall with a parapet down each side, and a driveway across the
// terrace's sidewalk at the top.
// Implements: REQ-CITY-038
function approach(r, at, y, quad, parapet) {
  const w = r.width, n = Math.max(2, Math.ceil(r.len / 0.25));
  for (let i = 0; i < n; i++) {
    const s0 = r.len * i / n, s1 = r.len * (i + 1) / n;
    quad(at(s0, 0, y(s0)), at(s1, 0, y(s1)), at(s1, w, y(s1)), at(s0, w, y(s0)), [0, s0, 3], [0, s1, 3], [w, s1, 3], [w, s0, 3], 1);
    for (const [side, inward] of [[0, 0.02], [w, w - 0.02]]) {
      quad(at(s0, side, r.y0), at(s1, side, r.y0), at(s1, side, y(s1) + parapet), at(s0, side, y(s0) + parapet),
        [s0, 0, 0], [s1, 0, 0], [s1, y(s1) + parapet - r.y0, 0], [s0, y(s0) + parapet - r.y0, 0], 0.68);
      quad(at(s0, inward, y(s0) + parapet), at(s1, inward, y(s1) + parapet), at(s1, side, y(s1) + parapet), at(s0, side, y(s0) + parapet),
        [s0, 0, 0], [s1, 0, 0], [s1, 0.02, 0], [s0, 0.02, 0], 0.95);
      quad(at(s0, inward, y(s0)), at(s1, inward, y(s1)), at(s1, inward, y(s1) + parapet), at(s0, inward, y(s0) + parapet),
        [s0, 0, 0], [s1, 0, 0], [s1, parapet, 0], [s0, parapet, 0], 0.8);
    }
  }
  const top = y(r.len);
  quad(at(r.len, 0, top), at(r.len + DRIVE, 0, top), at(r.len + DRIVE, w, top), at(r.len, w, top),
    [0, r.len, 2], [0, r.len + DRIVE, 2], [w, r.len + DRIVE, 2], [w, r.len, 2], 1);
}

// A flight of stone stairs from one plaza up to the next: its treads and risers, the
// wall along its open side with a parapet stepping up with it, and a landing level
// with the upper plaza. The walker goes up it as up a ramp (rampHeight); its steps
// are what it looks like. aRamp 0 throughout: stone, as the ramps' walls are, in a
// darker shade than theirs - the treads lighter than the risers, so the steps read.
function flight(r, at, quad, parapet) {
  const height = r.y1 - r.y0, n = Math.max(2, Math.ceil(height / STEP_RISE)), run = r.rise / n, w = r.width;
  const stone = s => [s, 0, 0];
  for (let i = 0; i < n; i++) {
    const s0 = run * i, s1 = run * (i + 1), y0 = r.y0 + height * i / n, y1 = r.y0 + height * (i + 1) / n;
    quad(at(s0, 0, y0), at(s0, w, y0), at(s0, w, y1), at(s0, 0, y1), stone(0), stone(w), stone(w), stone(0), 0.24); // riser
    quad(at(s0, 0, y1), at(s0, w, y1), at(s1, w, y1), at(s1, 0, y1), stone(s0), stone(s0), stone(s1), stone(s1), 0.36); // tread
    quad(at(s0, w, r.y0), at(s1, w, r.y0), at(s1, w, y1 + parapet), at(s0, w, y1 + parapet),
      stone(s0), stone(s1), stone(s1), stone(s0), 0.3); // the open side's wall
    quad(at(s0, w - 0.02, y1 + parapet), at(s1, w - 0.02, y1 + parapet), at(s1, w, y1 + parapet), at(s0, w, y1 + parapet),
      stone(s0), stone(s1), stone(s1), stone(s0), 0.42); // its parapet
  }
  quad(at(r.rise, 0, r.y1), at(r.rise, w, r.y1), at(r.len, w, r.y1), at(r.len, 0, r.y1),
    stone(r.rise), stone(r.rise), stone(r.len), stone(r.len), 0.36); // the landing
  quad(at(r.rise, w, r.y0), at(r.len, w, r.y0), at(r.len, w, r.y1 + parapet), at(r.rise, w, r.y1 + parapet),
    stone(r.rise), stone(r.len), stone(r.len), stone(r.rise), 0.3);
  quad(at(r.len, 0, r.y0), at(r.len, w, r.y0), at(r.len, w, r.y1 + parapet), at(r.len, 0, r.y1 + parapet),
    stone(0), stone(w), stone(w), stone(0), 0.3); // the landing's end
}

// The ramps' and bridges' look on top of a bendable material: asphalt with edge lines
// and chevrons pointing uphill, concrete walls - or, in the other styles, a copper
// track between the parts, or a lit gangway between the platforms.
// Implements: REQ-CITY-019, REQ-CITY-027
function rampMaterial(bendable) {
  const mat = bendable(new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.DoubleSide }));
  const bend = mat.onBeforeCompile;
  mat.onBeforeCompile = (shader, renderer) => {
    bend(shader, renderer);
    shader.vertexShader = 'attribute vec3 aRamp;\nvarying vec3 vRamp;\n' +
      shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\nvRamp = aRamp;');
    shader.fragmentShader = NOISE_GLSL + `
      uniform float uStyle;
      varying vec3 vRamp;
      float band(float t, float lo, float hi, float w) { return smoothstep(lo - w, lo + w, t) - smoothstep(hi - w, hi + w, t); }
    ` + shader.fragmentShader.replace('#include <color_fragment>', `
      #include <color_fragment>
      vec2 q = vRamp.xy;
      vec3 c;
      if (uStyle > 0.5) {
        // A track across the board, or a lit gangway out in the dark: the same deck,
        // with an edge and a line down the middle, in the style's own materials.
        float w = fwidth(q.x) + 1e-4;
        float wide = vRamp.z > 2.5 ? ${DECK_W} : ${RAMP_W};
        float mid = 1.0 - smoothstep(0.012 - w, 0.012 + w, abs(q.x - wide * 0.5));
        float edge = band(q.x, 0.0, 0.022, w) + band(q.x, wide - 0.022, wide, w);
        if (uStyle > 1.5) {
          c = vec3(0.05, 0.045, 0.1) * (0.8 + 0.4 * vnoise(q * 26.0));
          c += vec3(0.25, 0.8, 0.95) * (mid * 0.9 + edge * 0.5);
        } else {
          c = vec3(0.05, 0.16, 0.09) * (0.85 + 0.3 * vnoise(q * 26.0));
          c = mix(c, vec3(0.5, 0.3, 0.11), mid + edge * 0.8);
        }
      } else if (vRamp.z > 2.5) {
        // A bridge deck: asphalt with a dashed center line and edge lines.
        float w = fwidth(q.x) + 1e-4, wa = fwidth(q.y) + 1e-4;
        c = vec3(0.045, 0.047, 0.052) * (0.8 + 0.4 * vnoise(q * 24.0));
        c = mix(c, vec3(0.7), band(q.x, 0.03, 0.05, w) + band(q.x, ${DECK_W - 0.05}, ${DECK_W - 0.03}, w));
        c = mix(c, vec3(0.78, 0.6, 0.12), band(fract(q.y / 0.5), 0.0, 0.5, wa / 0.5) * band(q.x, ${DECK_W / 2 - 0.012}, ${DECK_W / 2 + 0.012}, w));
      } else if (vRamp.z > 1.5) {
        c = vec3(0.04, 0.042, 0.047) * (0.8 + 0.4 * vnoise(q * 30.0));
      } else if (vRamp.z > 0.5) {
        float w = fwidth(q.x) + 1e-4;
        c = vec3(0.04, 0.042, 0.047) * (0.8 + 0.4 * vnoise(q * 30.0));
        c = mix(c, vec3(0.75), band(q.x, 0.012, 0.022, w) + band(q.x, ${RAMP_W - 0.042}, ${RAMP_W - 0.032}, w));
        float ch = fract((q.y + abs(q.x - ${RAMP_W / 2}) * 0.8) / 0.55); // tips uphill
        c = mix(c, vec3(0.85, 0.85, 0.8), band(ch, 0.0, 0.1, fwidth(ch) + 1e-4) * step(abs(q.x - ${RAMP_W / 2}), ${RAMP_W / 4}));
      } else {
        c = vec3(0.5, 0.49, 0.47) * (0.85 + 0.3 * vnoise(q * vec2(20.0, 40.0)));
      }
      diffuseColor.rgb *= c * 2.2;`);
  };
  mat.customProgramCacheKey = () => 'bend-ramp';
  return mat;
}

// ------------------------------------------------------------------ props

// Lamps stand on the sidewalk along each terrace's edge (the street shader's SIDEWALK).
const TREE_SPACING = 1.1, LAMP_SPACING = 1.6, LAMP_INSET = 0.035, SHORE_INSET = 0.6;
// Parks: planted where the street shader draws lawn (farther than CARRIAGE + SIDEWALK
// from every obstacle, with a margin), off the gravel paths every PARK_PATHS.
const PARK_CLEAR = 1.14, PARK_PATHS = 2.2, PATH_CLEAR = 0.16, MAX_PARK_SAMPLES = 60000;
// Amenities: LAWN clear of everything around them, as the street shader draws lawn,
// and AMENITY_GAP clear of each other; tried at AMENITY_CHANCE of the places the park's
// paths cross, and placed where there is the room.
const LAWN = 0.94, AMENITY_GAP = 1.5, AMENITY_CHANCE = 0.08;

/**
 * The ground an amenity covers in the city, {w: along x, d: along z}, turned by `turn`
 * (0 or a quarter turn) and at its full size.
 */
export function amenitySpan(a, turn = 0) {
  const k = a.scale ?? 1, [w, d] = a.size;
  return turn ? { w: d * k, d: w * k } : { w: w * k, d: d * k };
}

/** The amenities of a style (amenitySpan's argument). */
export const amenitiesOf = style => (PROPS[style] || PROPS.city).amenities || [];

/**
 * What stands on the map: along every shore and in every park a tall prop and a low
 * one, along every terrace's edge a light, and the ramps and bridges - as meshes that
 * bend like the map. What those props are is the style's business (PROPS): trees,
 * bushes and street lamps in a city; capacitors, resistors and LEDs on a board;
 * crystals, glowing rubble and beacons in a galaxy. Returns a Group; the lights glow
 * at night (setNight).
 *
 * Implements: REQ-CITY-012, REQ-CITY-022, REQ-MAP-050
 */
export function makeProps(boxes, bendable, style = 'city') {
  const steps = dressing(boxes, bendable, style);
  for (;;) {
    const step = steps.next();
    if (step.done) return step.value;
  }
}

/**
 * makeProps a step at a time: the generator yields between boxes, park rows and
 * kinds of prop, and returns the Group. On a large map the whole of it takes longer
 * than several frames, and taken in one go it held the map still just after it had
 * been laid out (scene.js dressLater).
 *
 * Implements: REQ-PERF-015
 */
export function* dressing(boxes, bendable, style = 'city') {
  const set = dressed(style);
  const trees = [], bushes = [], lamps = [];
  const ramps = rampsFor(boxes), bridges = bridgesFor(boxes);
  const inDrive = drives(ramps);
  const onApproach = drives(ramps.filter(r => r.approach).map(r => ({ drive: r })));
  const tree = (x, z, y, r) => onApproach(x, z) || trees.push({ x, z, y, r, s: 0.7 + 0.6 * r, kind: Math.floor(rand(z * 3.1, x) * set.species.length) });
  const bush = (x, z, y, r) => onApproach(x, z) || bushes.push({ x, z, y, r, s: 0.7 + 0.7 * r });
  const amenities = [];
  const amenity = (x, z, y, kind, turn) => amenities.push({ x, z, y, turn, kind });
  for (const b of boxes) {
    const top = b.y + b.h;
    if (b.kind === 'land') {
      around(b, SHORE_INSET, TREE_SPACING, (x, z) => {
        const r = rand(x, z);
        if (r > 0.2) tree(x + (rand(z, x) - 0.5) * 0.3, z + (r - 0.5) * 0.3, top, r);
      });
      for (const inset of [0.28, 0.98]) {
        around(b, inset, 0.42, (x, z) => {
          const r = rand(x + inset, z);
          if (r > 0.45) bush(x + (rand(z, x + 1) - 0.5) * 0.12, z + (r - 0.5) * 0.12, top, r);
        });
      }
    } else if (b.kind === 'terrace' && b.node.kind !== 'file' && Math.min(b.w, b.d) > 2 * LAMP_INSET + 0.2) {
      around(b, LAMP_INSET, LAMP_SPACING, (x, z) => inDrive(x, z) || lamps.push({ x, z, y: top }));
    }
    yield;
  }
  yield* plantParks(boxes, tree, bush, set.amenities || [], amenity);

  const group = new THREE.Group();
  const m = new THREE.Matrix4(), q = new THREE.Quaternion(), p = new THREE.Vector3(), s = new THREE.Vector3();
  const cells = new Map(), scatters = [];
  const add = function* (geo, color, items, place, tint, options) {
    const material = bendable(new THREE.MeshBasicMaterial({ color, vertexColors: true, ...options }));
    const scatter = Object.create(Scatter.prototype);
    yield* scatter.build(geo, material, items, place, tint, cells);
    for (const mesh of scatter.meshes) {
      mesh.userData.day = color;
      group.add(mesh);
    }
    scatters.push(scatter);
    return scatter;
  };
  const plantAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.setFromAxisAngle(UP, it.r * 6.28), s.setScalar(it.s));
  const lampAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.identity(), s.setScalar(1));
  for (const [kind, t] of set.species.entries()) {
    const these = trees.filter(it => it.kind === kind);
    yield* add(t.stem, set.stem, these, plantAt);
    yield* add(t.head, '#ffffff', these, plantAt, set.tint(t.hue, t));
    for (const [r, opacity] of t.glow?.shells || []) {
      const glow = yield* add(glowShell(r, t.glow.at), t.glow.color, these, plantAt, null, {
        vertexColors: false, transparent: true, opacity,
        blending: THREE.AdditiveBlending, depthWrite: false,
      });
      for (const mesh of glow.meshes) mesh.userData.glow = opacity;
    }
  }
  const lows = set.lows || [{ head: set.low, hue: set.lowHue }];
  for (const [kind, low] of lows.entries()) {
    yield* add(low.head, '#ffffff', bushes.filter(it => Math.floor(it.r * 7919) % lows.length === kind), plantAt, set.tint(low.hue, low));
  }
  // Implements: REQ-CITY-039
  const amenityAt = (it, m) => m.compose(p.set(it.x, it.y, it.z), q.setFromAxisAngle(UP, it.turn), s.setScalar(set.amenities[it.kind].scale ?? 1));
  for (const [kind, a] of (set.amenities || []).entries()) {
    const these = amenities.filter(it => it.kind === kind);
    const scatter = yield* add(a.head, '#ffffff', these, amenityAt);
    for (const mesh of scatter.meshes) mesh.userData.amenity = kind;
    for (const [shell, opacity] of a.glow?.shells || []) {
      const glow = yield* add(shell, a.glow.color, these, amenityAt, null, {
        vertexColors: false, transparent: true, opacity, blending: THREE.AdditiveBlending, depthWrite: false,
      });
      for (const mesh of glow.meshes) Object.assign(mesh.userData, { glow: opacity, amenity: kind });
    }
  }
  yield* add(set.pole, set.poleColor, lamps, lampAt);
  for (const mesh of (yield* add(set.lampHead, set.headColor, lamps, lampAt)).meshes) mesh.userData.heads = true;
  group.userData.night = set.headNight;
  if (set.glowAt !== undefined) {
    for (const [r, opacity] of set.glow || GLOW) {
      const glow = yield* add(glowShell(r, set.glowAt), set.headNight, lamps, lampAt, null, {
        vertexColors: false, transparent: true, opacity,
        blending: THREE.AdditiveBlending, depthWrite: false,
      });
      for (const mesh of glow.meshes) {
        Object.assign(mesh.userData, { glow: opacity, afterDark: !!set.afterDark });
        mesh.visible = !set.afterDark; // setNight shows it, after dark
      }
    }
  }
  // A street lamp lights the pavement under it: a pool of its light, brightest under
  // the head and gone at its rim.
  // Implements: REQ-CITY-036
  if (set.pool) {
    const pool = yield* add(lightPool(set.pool), set.headNight, lamps, lampAt, null, {
      transparent: true, opacity: POOL, blending: THREE.AdditiveBlending, depthWrite: false,
    });
    for (const mesh of pool.meshes) {
      Object.assign(mesh.userData, { glow: POOL, afterDark: true });
      mesh.visible = false;
    }
  }
  yield;
  group.userData.lod = { cells, scatters };
  // What a walker cannot walk through. A trunk, a capacitor's case, a crystal and a
  // lamp post are all a circle standing on a spot; a bush is something to walk over.
  group.userData.obstacles = [
    ...trees.map(it => ({ x: it.x, z: it.z, y: it.y, r: set.solid * it.s })),
    ...lamps.map(it => ({ x: it.x, z: it.z, y: it.y, r: set.post })),
    // A goalpost, a hoop's pole, a swing's legs: turned with what they belong to.
    ...amenities.flatMap(it => {
      const a = set.amenities[it.kind], k = a.scale ?? 1;
      return (a.posts || []).map(([px, pz, r]) => {
        const c = Math.cos(it.turn), sn = Math.sin(it.turn);
        return { x: it.x + (px * c + pz * sn) * k, z: it.z + (-px * sn + pz * c) * k, y: it.y, r: r * k };
      });
    }),
  ];
  const surfaces = [];
  if (ramps.length) surfaces.push(yield* rampGeometry(ramps));
  if (bridges.length) surfaces.push(yield* bridgeGeometry(bridges));
  for (const geo of surfaces) {
    const mesh = new THREE.Mesh(geo, rampMaterial(bendable));
    mesh.frustumCulled = false;
    mesh.userData.day = '#ffffff';
    group.add(mesh);
  }
  return group;
}

// Samples each block's lawn (the park the street shader draws) on a jittered grid
// and plants trees and bushes there, yielding after every row and every building
// filed for the distance test. First, some stretches of lawn that are lawn all over
// are given to the style's amenities (`kinds`, placed with `amenity`), at their size,
// and nothing is planted on those.
// Implements: REQ-CITY-024, REQ-CITY-039
function* plantParks(boxes, tree, bush, kinds, amenity) {
  const all = blocks(boxes);
  yield;
  const area = [...all.keys()].reduce((a, t) => a + t.w * t.d, 0);
  const step = Math.max(0.6, Math.sqrt(area / MAX_PARK_SAMPLES));
  const offPath = v => Math.abs(((v / PARK_PATHS) % 1 + 1) % 1 - 0.5) * PARK_PATHS > PATH_CLEAR;
  for (const [t, kids] of all) {
    const top = t.y + t.h;
    const x0 = t.x - t.w / 2 + PARK_CLEAR, x1 = t.x + t.w / 2 - PARK_CLEAR;
    const z0 = t.z - t.d / 2 + PARK_CLEAR, z1 = t.z + t.d / 2 - PARK_CLEAR;
    if (x1 <= x0 || z1 <= z0) continue;
    // Kids by grid cell, for the distance test: one array over the block's cells,
    // which a block of a hundred thousand of them fills far quicker than a Map would.
    const cell = 1.5;
    const i0 = Math.floor((t.x - t.w / 2) / cell) - 1, j0 = Math.floor((t.z - t.d / 2) / cell) - 1;
    const ni = Math.floor((t.x + t.w / 2) / cell) + 2 - i0, nj = Math.floor((t.z + t.d / 2) / cell) + 2 - j0;
    const grid = new Array(ni * nj);
    const at = (i, j) => (i < i0 || j < j0 || i >= i0 + ni || j >= j0 + nj ? -1 : (i - i0) * nj + (j - j0));
    for (const k of kids) {
      for (let i = Math.floor((k.x - k.w / 2) / cell); i <= Math.floor((k.x + k.w / 2) / cell); i++) {
        for (let j = Math.floor((k.z - k.d / 2) / cell); j <= Math.floor((k.z + k.d / 2) / cell); j++) {
          const n = at(i, j);
          if (n >= 0) (grid[n] ||= []).push(k);
        }
      }
      yield;
    }
    const clear = (x, z) => {
      const i = Math.floor(x / cell), j = Math.floor(z / cell);
      for (let offsetI = -1; offsetI <= 1; offsetI++) for (let offsetJ = -1; offsetJ <= 1; offsetJ++) {
        for (const k of grid[at(i + offsetI, j + offsetJ)] || []) {
          if (rectDist({ x0: x, x1: x, z0: z, z1: z }, k) < PARK_CLEAR) return false;
        }
      }
      return true;
    };
    // A stretch of park is lawn all over when it is LAWN from the block's edge and
    // from everything standing on it, which is where the street shader stops drawing
    // street - and clear of the amenities already placed, by a few steps.
    const placed = [];
    const roomy = r => {
      if (r.x0 < t.x - t.w / 2 + LAWN || r.x1 > t.x + t.w / 2 - LAWN || r.z0 < t.z - t.d / 2 + LAWN || r.z1 > t.z + t.d / 2 - LAWN) return false;
      if (placed.some(o => rectDist(r, o) < AMENITY_GAP)) return false;
      for (let i = Math.floor((r.x0 - LAWN) / cell); i <= Math.floor((r.x1 + LAWN) / cell); i++) {
        for (let j = Math.floor((r.z0 - LAWN) / cell); j <= Math.floor((r.z1 + LAWN) / cell); j++) {
          for (const k of grid[at(i, j)] || []) if (rectDist(r, k) < LAWN) return false;
        }
      }
      return true;
    };
    // Tried where the paths cross, each place once; which kind, and which way round,
    // is the place's own, so a map dresses the same every time.
    for (let kz = Math.ceil(z0 / PARK_PATHS); kinds.length && kz * PARK_PATHS <= z1; kz++) {
      for (let kx = Math.ceil(x0 / PARK_PATHS); kx * PARK_PATHS <= x1; kx++) {
        const cx = kx * PARK_PATHS, cz = kz * PARK_PATHS;
        if (rand(cx * 0.37 + 11.3, cz * 0.53 - 7.1) >= AMENITY_CHANCE) continue;
        const kind = Math.floor(rand(cx * 1.13 - 4.1, cz * 0.91 + 9.7) * kinds.length);
        const turn = rand(cz + 3.7, cx) < 0.5 ? 0 : Math.PI / 2;
        const { w, d } = amenitySpan(kinds[kind], turn);
        const r = { x0: cx - w / 2, x1: cx + w / 2, z0: cz - d / 2, z1: cz + d / 2 };
        if (!roomy(r)) continue;
        amenity(cx, cz, top, kind, turn);
        placed.push(r);
      }
    }
    // Nothing planted on one, nor right up against it.
    const onAmenity = (x, z) => placed.some(r => x > r.x0 - 0.3 && x < r.x1 + 0.3 && z > r.z0 - 0.3 && z < r.z1 + 0.3);
    yield;
    for (let z = z0; z <= z1; z += step) {
      for (let x = x0; x <= x1; x += step) {
        const px = x + (rand(x, z) - 0.5) * step * 0.8, pz = z + (rand(z, x) - 0.5) * step * 0.8;
        if (px < x0 || px > x1 || pz < z0 || pz > z1 || !offPath(px) || !offPath(pz) || !clear(px, pz)) continue;
        if (placed.length && onAmenity(px, pz)) continue;
        const r = rand(px * 1.7, pz * 2.3);
        if (r < 0.16) tree(px, pz, top, r / 0.16);
        else if (r < 0.5) bush(px, pz, top, (r - 0.16) / 0.34);
      }
      yield;
    }
  }
}

/** Day or night for the props: the lights come on, everything else darkens. */
export function setNight(group, night) {
  for (const mesh of group.children) {
    // A glow is the one thing that does not go down with the light: it is the
    // light. It is drawn a little stronger in the dark, as one looks.
    if (mesh.userData.glow) mesh.material.opacity = mesh.userData.glow * (night ? 1.5 : 1);
    // A street lamp is off by day: its glow, and the pool it throws, are night's alone.
    if (mesh.userData.afterDark) mesh.visible = night;
    else if (mesh.userData.glow) continue;
    else if (mesh.userData.heads && night) mesh.material.color.set(group.userData.night);
    else mesh.material.color.set(mesh.userData.day).multiplyScalar(night ? 0.4 : 1);
  }
}

const UP = new THREE.Vector3(0, 1, 0);

// Whether a point is on a ramp's driveway (or within 0.2 of one), where no lamp
// stands. The driveways are filed by the cells of a grid they reach, so a point is
// held against the few near it rather than against every ramp on the map.
const DRIVE_CELL = 4;
function drives(ramps) {
  const grid = new Map();
  for (const { drive } of ramps) {
    if (!drive) continue;
    for (let i = Math.floor((drive.x0 - 0.2) / DRIVE_CELL); i <= Math.floor((drive.x1 + 0.2) / DRIVE_CELL); i++) {
      for (let j = Math.floor((drive.z0 - 0.2) / DRIVE_CELL); j <= Math.floor((drive.z1 + 0.2) / DRIVE_CELL); j++) {
        const key = i + ',' + j;
        if (!grid.has(key)) grid.set(key, []);
        grid.get(key).push(drive);
      }
    }
  }
  return (x, z) => (grid.get(Math.floor(x / DRIVE_CELL) + ',' + Math.floor(z / DRIVE_CELL)) || [])
    .some(d => x > d.x0 - 0.2 && x < d.x1 + 0.2 && z > d.z0 - 0.2 && z < d.z1 + 0.2);
}

// Calls fn at points spaced along a rectangle inset from a box's edges.
function around(b, inset, spacing, callback) {
  const x0 = b.x - b.w / 2 + inset, x1 = b.x + b.w / 2 - inset, z0 = b.z - b.d / 2 + inset, z1 = b.z + b.d / 2 - inset;
  if (x1 <= x0 || z1 <= z0) return;
  const side = (ax, az, bx, bz) => {
    const n = Math.max(1, Math.round(Math.hypot(bx - ax, bz - az) / spacing));
    for (let i = 0; i < n; i++) callback(ax + (bx - ax) * i / n, az + (bz - az) * i / n, i);
  };
  side(x0, z0, x1, z0); side(x1, z0, x1, z1); side(x1, z1, x0, z1); side(x0, z1, x0, z0);
}

// Deterministic 0..1 from a position, so props stay put across relayouts.
function rand(x, z) {
  const v = Math.sin(x * 12.9898 + z * 78.233) * 43758.5453;
  return v - Math.floor(v);
}

// Geometries with vertex colors as fixed shading: lighter facing up and towards the
// light, darker towards the base (self-shadowing), with a little per-vertex jitter so
// foliage does not look faceted.
// Implements: REQ-CITY-022
function shaded(geo, jitter = 0) {
  geo = geo.index ? geo.toNonIndexed() : geo;
  geo.computeVertexNormals();
  geo.computeBoundingBox();
  const { min, max } = geo.boundingBox;
  const n = geo.getAttribute('normal'), position = geo.getAttribute('position'), colors = [];
  for (let i = 0; i < n.count; i++) {
    const up = (position.getY(i) - min.y) / Math.max(1e-6, max.y - min.y);
    const k = (0.5 + 0.4 * Math.max(0, n.getY(i)) + 0.12 * n.getX(i) - 0.06 * n.getZ(i)) * (0.72 + 0.28 * up)
      * (1 + jitter * (rand(position.getX(i) * 7.1, position.getZ(i) * 5.3 + position.getY(i)) - 0.5));
    colors.push(k, k, k);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  return geo;
}

// One non-indexed geometry from several (position and color only, as shaded makes them).
function merge(geos) {
  const out = new THREE.BufferGeometry();
  for (const name of ['position', 'color']) {
    const parts = geos.map(g => g.getAttribute(name).array);
    const all = new Float32Array(parts.reduce((a, p) => a + p.length, 0));
    let o = 0;
    for (const p of parts) { all.set(p, o); o += p.length; }
    out.setAttribute(name, new THREE.BufferAttribute(all, 3));
  }
  return out;
}

const blob = (r, x, y, z, sy = 1) => shaded(new THREE.IcosahedronGeometry(r, 1).scale(1, sy, 1).translate(x, y, z), 0.25);
const trunk = (h, r) => shaded(new THREE.CylinderGeometry(r * 0.7, r, h, 6).translate(0, h / 2, 0));

// Tree species: a broadleaf with a crown of several blobs, a conifer of stacked cones,
// and a slender poplar. hue: their foliage's base hue. stem and head are the trunk and
// the crown under the names every style's species share, which is what makeProps reads.
// Implements: REQ-CITY-022
const TREES = [
  {
    stem: trunk(0.3, 0.04), hue: 0.24,
    head: merge([blob(0.19, 0, 0.47, 0), blob(0.14, 0.12, 0.4, 0.05), blob(0.14, -0.1, 0.42, -0.07), blob(0.12, 0.02, 0.6, -0.04)]),
  },
  {
    stem: trunk(0.14, 0.035), hue: 0.3,
    head: merge([0, 1, 2].map(i => shaded(new THREE.ConeGeometry(0.2 - i * 0.05, 0.3, 8).translate(0, 0.25 + i * 0.16, 0), 0.2))),
  },
  { stem: trunk(0.2, 0.03), hue: 0.21, head: merge([blob(0.12, 0, 0.5, 0, 2.6)]) },
];
const BUSH = merge([blob(0.085, 0, 0.055, 0, 0.8), blob(0.065, 0.07, 0.04, 0.03, 0.8), blob(0.06, -0.06, 0.04, -0.04, 0.8)]);
const POLE = shaded(new THREE.CylinderGeometry(0.012, 0.018, 0.6, 5).translate(0, 0.3, 0));
const HEAD = shaded(new THREE.BoxGeometry(0.07, 0.025, 0.05).translate(0, 0.61, 0));

// ------------------------------------------------------------------ the other styles' props

const cyl = (r0, r1, h, y, segments = 10) => shaded(new THREE.CylinderGeometry(r0, r1, h, segments).translate(0, y + h / 2, 0));
const legs = (h, dx) => merge([
  shaded(new THREE.CylinderGeometry(0.008, 0.008, h, 5).translate(dx, h / 2, 0)),
  shaded(new THREE.CylinderGeometry(0.008, 0.008, h, 5).translate(-dx, h / 2, 0)),
]);

const box = (w, h, d, x, y, z) => shaded(new THREE.BoxGeometry(w, h, d).translate(x, y + h / 2, z));

// Parts soldered to the board, two or three of each kind in the shapes they are sold
// in: capacitors - a tall electrolytic with its vent cross, a squat one, a ceramic
// disc, a film box; transistors - a small TO-92 and a TO-220 with its tab; inductors -
// a wound drum and a toroid. The legs, and the TO-220's tab, are the stem, in tin.
// light: how bright the part's color is, for the light ones.
// Implements: REQ-MAP-055
const PARTS = [
  {
    hue: 0.62, // the dark blue a capacitor's sleeve usually is
    stem: legs(0.1, 0.022),
    head: merge([
      cyl(0.085, 0.085, 0.38, 0.1),
      shaded(new THREE.CylinderGeometry(0.087, 0.087, 0.02, 12).translate(0, 0.475, 0)),  // the crimp
      shaded(new THREE.BoxGeometry(0.03, 0.004, 0.17).translate(0, 0.487, 0)),            // the vent cross
      shaded(new THREE.BoxGeometry(0.17, 0.004, 0.03).translate(0, 0.487, 0)),
    ]),
  },
  {
    hue: 0.0,
    stem: legs(0.06, 0.03),
    head: merge([
      cyl(0.11, 0.11, 0.2, 0.06, 14),
      shaded(new THREE.CylinderGeometry(0.112, 0.112, 0.016, 14).translate(0, 0.23, 0)),
      box(0.025, 0.2, 0.012, 0, 0.06, 0.105),                                             // the minus stripe
    ]),
  },
  {
    hue: 0.08, light: 0.2, // a ceramic disc's glaze
    stem: legs(0.14, 0.03),
    head: shaded(new THREE.CylinderGeometry(0.075, 0.075, 0.03, 14).rotateX(Math.PI / 2).translate(0, 0.2, 0)),
  },
  {
    hue: 0.14, light: 0.32, // a film capacitor's yellow box
    stem: legs(0.04, 0.05),
    head: merge([box(0.17, 0.13, 0.08, 0, 0.04, 0), box(0.172, 0.01, 0.082, 0, 0.16, 0)]),
  },
  {
    hue: 0.0,
    stem: legs(0.08, 0.016),
    head: merge([
      shaded(new THREE.CylinderGeometry(0.075, 0.075, 0.16, 12, 1, false, -Math.PI / 2, Math.PI).translate(0, 0.16, 0)),
      shaded(new THREE.BoxGeometry(0.15, 0.16, 0.03).translate(0, 0.16, 0.012)),
    ]),
  },
  {
    hue: 0.0,
    stem: merge([
      legs(0.12, 0.05), cyl(0.008, 0.008, 0.12, 0, 5),
      box(0.17, 0.13, 0.014, 0, 0.3, -0.022),                                             // the tab, with its screw
      shaded(new THREE.CylinderGeometry(0.024, 0.024, 0.02, 8).rotateX(Math.PI / 2).translate(0, 0.365, -0.022)),
    ]),
    head: merge([box(0.17, 0.18, 0.05, 0, 0.12, 0), box(0.04, 0.004, 0.052, 0, 0.3, 0)]),
  },
  {
    hue: 0.09,
    stem: legs(0.07, 0.03),
    head: merge([
      cyl(0.06, 0.06, 0.24, 0.07, 8),
      ...[0, 1, 2, 3].map(i => shaded(new THREE.TorusGeometry(0.065, 0.014, 5, 10).rotateX(Math.PI / 2).translate(0, 0.11 + i * 0.055, 0))),
    ]),
  },
  {
    hue: 0.07, light: 0.18, // copper wound round a ring
    stem: legs(0.05, 0.06),
    head: merge([
      shaded(new THREE.TorusGeometry(0.1, 0.045, 8, 18).translate(0, 0.17, 0)),
      ...Array.from({ length: 12 }, (_, i) => shaded(new THREE.TorusGeometry(0.05, 0.008, 4, 8)
        .rotateX(Math.PI / 2).translate(0.1, 0, 0).rotateZ(i * Math.PI / 6).translate(0, 0.17, 0))),
    ]),
  },
];
// The low ones, lying on their pads: a resistor with tin ends, a ceramic capacitor,
// a SOT-23 transistor on its three feet and a shielded power inductor.
const SMD = [
  {
    hue: 0.09,
    head: merge([box(0.13, 0.05, 0.07, 0, 0, 0), box(0.03, 0.052, 0.072, 0.055, 0, 0), box(0.03, 0.052, 0.072, -0.055, 0, 0)]),
  },
  { hue: 0.08, light: 0.2, head: merge([box(0.1, 0.05, 0.06, 0, 0, 0), box(0.022, 0.052, 0.062, 0.04, 0, 0), box(0.022, 0.052, 0.062, -0.04, 0, 0)]) },
  {
    hue: 0.0,
    head: merge([box(0.1, 0.035, 0.06, 0, 0.012, 0),
      box(0.016, 0.012, 0.03, -0.03, 0, 0.042), box(0.016, 0.012, 0.03, 0.03, 0, 0.042), box(0.016, 0.012, 0.03, 0, 0, -0.042)]),
  },
  { hue: 0.6, head: merge([box(0.14, 0.06, 0.14, 0, 0, 0), cyl(0.045, 0.045, 0.004, 0.06, 12)]) },
];
const LED_LEGS = merge([
  shaded(new THREE.CylinderGeometry(0.009, 0.009, 0.42, 5).translate(0.016, 0.21, 0)),
  shaded(new THREE.CylinderGeometry(0.009, 0.009, 0.36, 5).translate(-0.016, 0.18, 0)),
]);
const LED = merge([
  shaded(new THREE.CylinderGeometry(0.038, 0.042, 0.08, 10).translate(0, 0.46, 0)),
  shaded(new THREE.SphereGeometry(0.038, 10, 6, 0, Math.PI * 2, 0, Math.PI / 2).translate(0, 0.54, 0)),
]);
// What a lit LED throws around itself. Nothing on this map is lit and there is no
// pass to bloom in, so a glow has to be geometry: shells of light over the lens,
// added to whatever is behind them, the wider one fainter. Two is enough to read as
// one soft ball at this size, and they cost two draws however many lights there are.
const GLOW = [[0.075, 0.55], [0.135, 0.22], [0.24, 0.07]];
// How bright a street lamp's pool of light is at its middle, before night's 1.5.
const POOL = 0.32;
const pools = new Map();
// A disc on the ground, white in the middle and black at the rim: added to the street
// under it, the black adds nothing, so the pool fades out to its edge. Lifted clear of
// the pavement it lies on.
function lightPool(r) {
  if (!pools.has(r)) {
    const geometry = new THREE.CircleGeometry(r, 24).rotateX(-Math.PI / 2).translate(0, 0.012, 0);
    const position = geometry.getAttribute('position'), colors = [];
    for (let i = 0; i < position.count; i++) {
      const k = Math.max(0, 1 - Math.hypot(position.getX(i), position.getZ(i)) / r) ** 1.6;
      colors.push(k, k, k);
    }
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    pools.set(r, geometry);
  }
  return pools.get(r);
}
// One shell of each size and height for every layout: the props' instanced meshes
// share their geometry across layouts and do not dispose of it (scene.js dropProps),
// and lod.js keeps its coarse copies by geometry.
const shells = new Map();
const glowShell = (r, y) => {
  const key = r + ',' + y;
  if (!shells.has(key)) shells.set(key, new THREE.SphereGeometry(r, 9, 6).translate(0, y, 0));
  return shells.get(key);
};

// What grows out in the dark: spires and clusters of crystal, fungi and seed pods
// that light themselves, ringed stones and slabs of obsidian; low down, rubble,
// sprouts of crystal and cinders. A part that lights itself carries its glow: the
// height its shells sit at, their color, and their sizes and strengths (as GLOW).
// Implements: REQ-MAP-057
const shard = (r, h, x, y, z, tilt = 0) => shaded(
  new THREE.ConeGeometry(r, h, 5).rotateZ(tilt).translate(x, y + h / 2, z), 0.15);
const CRYSTALS = [
  { hue: 0.72, stem: shard(0.06, 0.1, 0, 0, 0), head: merge([shard(0.09, 0.62, 0, 0.04, 0), shard(0.05, 0.3, 0.08, 0.02, 0.05, 0.3)]) },
  {
    hue: 0.52, stem: shard(0.07, 0.08, 0, 0, 0),
    head: merge([shard(0.07, 0.34, 0.06, 0.02, 0.02, 0.35), shard(0.06, 0.44, -0.05, 0.02, -0.03, -0.28), shard(0.05, 0.26, 0.01, 0.02, -0.08, 0.12)]),
  },
  { hue: 0.85, stem: shard(0.09, 0.07, 0, 0, 0), head: merge([shard(0.13, 0.28, 0, 0.03, 0)]) },
  {
    hue: 0.45, // a fungus that lights its own cap
    stem: shaded(new THREE.CylinderGeometry(0.025, 0.04, 0.3, 7).translate(0, 0.15, 0)),
    head: merge([
      shaded(new THREE.SphereGeometry(0.15, 12, 6, 0, Math.PI * 2, 0, Math.PI / 2).scale(1, 0.55, 1).translate(0, 0.29, 0), 0.1),
      shaded(new THREE.CylinderGeometry(0.15, 0.05, 0.03, 12).translate(0, 0.275, 0)),
      blob(0.03, 0.06, 0.37, 0.03), blob(0.022, -0.07, 0.35, -0.02),
    ]),
    glow: { at: 0.31, color: '#8dffd8', shells: [[0.165, 0.09], [0.24, 0.03]] },
  },
  {
    hue: 0.11, // a seed pod on a stalk, lit from within
    stem: shaded(new THREE.CylinderGeometry(0.008, 0.016, 0.5, 5).rotateZ(0.08).translate(0.02, 0.25, 0)),
    head: merge([
      shaded(new THREE.IcosahedronGeometry(0.06, 1).scale(1, 1.3, 1).translate(0, 0.56, 0)),
      shard(0.025, 0.12, 0.05, 0.2, 0, -0.9), shard(0.02, 0.1, -0.05, 0.3, 0.02, 0.9),
    ]),
    glow: { at: 0.56, color: '#ffc46b', shells: [[0.09, 0.45], [0.16, 0.18], [0.27, 0.06]] },
  },
  {
    hue: 0.07, // a stone with a ring round it, on a plinth
    stem: shaded(new THREE.CylinderGeometry(0.03, 0.07, 0.12, 6).translate(0, 0.06, 0)),
    head: merge([
      shaded(new THREE.IcosahedronGeometry(0.11, 1).translate(0, 0.25, 0), 0.25),
      shaded(new THREE.TorusGeometry(0.19, 0.012, 4, 24).rotateX(Math.PI / 2 - 0.4).rotateZ(0.3).translate(0, 0.25, 0)),
    ]),
  },
  {
    hue: 0.68, // a slab of obsidian standing on end
    stem: shaded(new THREE.BoxGeometry(0.16, 0.04, 0.09).translate(0, 0.02, 0)),
    head: shaded(new THREE.BoxGeometry(0.1, 0.62, 0.035).rotateZ(0.06).translate(0, 0.34, 0), 0.1),
  },
];
const RUBBLE = merge([blob(0.07, 0, 0.045, 0, 0.7), blob(0.05, 0.06, 0.03, 0.02, 0.7), blob(0.045, -0.05, 0.03, -0.04, 0.7)]);
const DEBRIS = [
  { hue: 0.68, head: RUBBLE },
  { hue: 0.55, head: merge([shard(0.03, 0.14, 0, 0, 0, 0.2), shard(0.025, 0.1, 0.04, 0, 0.02, -0.4), shard(0.02, 0.08, -0.035, 0, -0.02, 0.5)]) },
  { hue: 0.03, head: merge([blob(0.05, 0, 0.03, 0, 0.6), blob(0.025, 0.06, 0.015, -0.03, 0.6)]) },
];
const BEACON_STEM = shaded(new THREE.CylinderGeometry(0.008, 0.016, 0.44, 5).translate(0, 0.22, 0));
const BEACON = merge([
  shaded(new THREE.IcosahedronGeometry(0.055, 1).translate(0, 0.52, 0)),
  shaded(new THREE.TorusGeometry(0.075, 0.006, 5, 14).rotateX(Math.PI / 2).translate(0, 0.52, 0)),
]);

// ------------------------------------------------------------------ amenities

// What a park holds besides its trees: in a city a five-a-side pitch, a basketball
// court and a playground; on a board a pin header, a heat sink and a coin cell with
// its crystal; in the galaxy a landing pad, a dish listening to the sky and a ring
// of standing stones round a lit crystal. Each fills one square of lawn between the
// park's gravel paths (plantParks), modeled along x about its middle with the
// ground at 0, and says how much ground it covers as modeled (size, [along x, along z]),
// how much it is enlarged to stand true to the walker - who is about half a unit tall,
// so a unit is some three and a half meters (scale) - and where it stands in the
// walker's way as modeled: posts, [x, z, radius].
// Implements: REQ-CITY-039
const painted = (geo, hex, jitter = 0) => {
  geo = shaded(geo, jitter);
  const c = new THREE.Color(hex), colors = geo.getAttribute('color');
  for (let i = 0; i < colors.count; i++) colors.setXYZ(i, colors.getX(i) * c.r, colors.getY(i) * c.g, colors.getZ(i) * c.b);
  return geo;
};
// Flat on the ground: a patch of surface, a painted line, a painted ring.
const LIFT = 0.006, PAINT = 0.009;
const patch = (w, d, hex, x = 0, z = 0) => painted(new THREE.PlaneGeometry(w, d).rotateX(-Math.PI / 2).translate(x, LIFT, z), hex);
const stripe = (x0, z0, x1, z1, hex, width = 0.018) => {
  const len = Math.hypot(x1 - x0, z1 - z0);
  return painted(new THREE.PlaneGeometry(len, width).rotateX(-Math.PI / 2).rotateY(-Math.atan2(z1 - z0, x1 - x0))
    .translate((x0 + x1) / 2, PAINT, (z0 + z1) / 2), hex);
};
const outline = (w, d, hex, x = 0, z = 0, width) => merge([
  stripe(x - w / 2, z - d / 2, x + w / 2, z - d / 2, hex, width), stripe(x - w / 2, z + d / 2, x + w / 2, z + d / 2, hex, width),
  stripe(x - w / 2, z - d / 2, x - w / 2, z + d / 2, hex, width), stripe(x + w / 2, z - d / 2, x + w / 2, z + d / 2, hex, width),
]);
const ring = (r, hex, x = 0, z = 0, start = 0, sweep = Math.PI * 2, width = 0.018) => painted(
  new THREE.RingGeometry(r - width / 2, r + width / 2, 28, 1, start, sweep).rotateX(-Math.PI / 2).translate(x, PAINT, z), hex);
// Standing: a post, a bar between two points, a block.
const post = (x, z, h, r, hex, y = 0) => painted(new THREE.CylinderGeometry(r, r, h, 6).translate(x, y + h / 2, z), hex);
const bar = (a, b, r, hex) => {
  const from = new THREE.Vector3(...a), to = new THREE.Vector3(...b), len = from.distanceTo(to);
  const geo = new THREE.CylinderGeometry(r, r, len, 5);
  geo.applyQuaternion(new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), to.clone().sub(from).normalize()));
  return painted(geo.translate((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2), hex);
};
const block = (w, h, d, x, y, z, hex) => painted(new THREE.BoxGeometry(w, h, d).translate(x, y + h / 2, z), hex);
// The halo round whatever on one lights itself, as an LED's (GLOW): a shell at each
// point, for each [radius, strength].
const lit = (points, shells) => shells.map(([r, opacity]) => [
  merge(points.map(([x, y, z]) => shaded(new THREE.SphereGeometry(r, 9, 6).translate(x, y, z)))), opacity]);
const PAD_LAMPS = Array.from({ length: 6 }, (_, i) => [Math.cos(i * Math.PI / 3) * 0.72, 0.11, Math.sin(i * Math.PI / 3) * 0.72]);

// A goal: two posts and a crossbar on the line, the net sloping back to the ground.
const goal = side => {
  const x = side * 0.85, back = side * 0.95, white = '#f2f2ee', net = '#c9ccd0';
  return merge([
    post(x, -0.13, 0.12, 0.007, white), post(x, 0.13, 0.12, 0.007, white), bar([x, 0.12, -0.13], [x, 0.12, 0.13], 0.007, white),
    bar([x, 0.12, -0.13], [back, 0, -0.13], 0.004, net), bar([x, 0.12, 0.13], [back, 0, 0.13], 0.004, net),
    painted(new THREE.PlaneGeometry(0.26, Math.hypot(0.1, 0.12)).rotateY(Math.PI / 2)
      .rotateZ(side * Math.atan2(0.1, 0.12)).translate((x + back) / 2, 0.06, 0), net),
  ]);
};
// A hoop on its pole, the board facing in across the court.
const hoop = side => {
  const x = side * 0.68, white = '#f4f4f0', orange = '#e06a1c';
  return merge([
    post(x, 0, 0.3, 0.01, '#3b4148'), bar([x, 0.28, 0], [side * 0.6, 0.28, 0], 0.007, '#3b4148'),
    block(0.012, 0.1, 0.17, side * 0.6, 0.24, 0, white),
    painted(new THREE.TorusGeometry(0.028, 0.004, 4, 14).rotateX(Math.PI / 2).translate(side * 0.565, 0.25, 0), orange),
  ]);
};
const CITY_AMENITIES = [
  { // five-a-side
    head: merge([
      patch(1.78, 1.18, '#4c9a42'), patch(0.3, 1.18, '#57a64b', -0.6), patch(0.3, 1.18, '#57a64b', 0), patch(0.3, 1.18, '#57a64b', 0.6),
      outline(1.7, 1.1, '#f4f4ee'), stripe(0, -0.55, 0, 0.55, '#f4f4ee'), ring(0.15, '#f4f4ee'),
      outline(0.24, 0.5, '#f4f4ee', -0.73), outline(0.24, 0.5, '#f4f4ee', 0.73), goal(-1), goal(1),
    ]),
    // Goals 2 m high: a small-sided pitch about 30 m by 20.
    size: [1.92, 1.2], scale: 4.5,
    posts: [[-0.85, -0.13, 0.03], [-0.85, 0.13, 0.03], [0.85, -0.13, 0.03], [0.85, 0.13, 0.03]],
  },
  { // a basketball court
    head: merge([
      patch(1.5, 0.98, '#b4573c'), patch(0.26, 0.28, '#d07a52', -0.6), patch(0.26, 0.28, '#d07a52', 0.6),
      outline(1.36, 0.86, '#f4f4ee'), stripe(0, -0.43, 0, 0.43, '#f4f4ee'), ring(0.13, '#f4f4ee'),
      outline(0.26, 0.28, '#f4f4ee', -0.55), outline(0.26, 0.28, '#f4f4ee', 0.55),
      ring(0.36, '#f4f4ee', -0.68, 0, -Math.PI / 2, Math.PI), ring(0.36, '#f4f4ee', 0.68, 0, Math.PI / 2, Math.PI),
      hoop(-1), hoop(1),
    ]),
    // The rim at regulation height, 3 m: a court about 18 m by 12.
    size: [1.5, 1.0], scale: 3.5,
    posts: [[-0.68, 0, 0.03], [0.68, 0, 0.03]],
  },
  { // a playground: swings, a slide and a seesaw, in a sandpit
    head: merge([
      patch(1.5, 1.3, '#d9c28c'), outline(1.5, 1.3, '#8a6d45', 0, 0, 0.03),
      // the swings
      bar([-0.55, 0, -0.42], [-0.45, 0.32, -0.42], 0.009, '#2f6fb3'), bar([-0.35, 0, -0.42], [-0.45, 0.32, -0.42], 0.009, '#2f6fb3'),
      bar([-0.55, 0, 0.12], [-0.45, 0.32, 0.12], 0.009, '#2f6fb3'), bar([-0.35, 0, 0.12], [-0.45, 0.32, 0.12], 0.009, '#2f6fb3'),
      bar([-0.45, 0.32, -0.44], [-0.45, 0.32, 0.14], 0.009, '#2f6fb3'),
      ...[-0.28, -0.02].flatMap(z => [
        bar([-0.45, 0.32, z - 0.04], [-0.45, 0.08, z - 0.04], 0.002, '#8d9196'), bar([-0.45, 0.32, z + 0.04], [-0.45, 0.08, z + 0.04], 0.002, '#8d9196'),
        block(0.06, 0.012, 0.1, -0.45, 0.07, z, '#c23b2c'),
      ]),
      // the slide: its ladder, its platform and the chute down
      bar([0.18, 0, -0.3], [0.2, 0.24, -0.3], 0.007, '#e3b324'), bar([0.18, 0, -0.18], [0.2, 0.24, -0.18], 0.007, '#e3b324'),
      ...[0.06, 0.12, 0.18].map(y => bar([0.185 + y * 0.08, y, -0.3], [0.185 + y * 0.08, y, -0.18], 0.004, '#e3b324')),
      block(0.12, 0.015, 0.14, 0.26, 0.24, -0.24, '#c23b2c'),
      painted(new THREE.BoxGeometry(0.42, 0.012, 0.11).rotateZ(-Math.atan2(0.22, 0.38)).translate(0.51, 0.13, -0.24), '#e3b324'),
      // the seesaw
      block(0.04, 0.05, 0.04, 0.35, 0, 0.32, '#3b4148'),
      painted(new THREE.BoxGeometry(0.5, 0.012, 0.05).rotateZ(0.12).translate(0.35, 0.06, 0.32), '#c23b2c'),
    ]),
    // Swings about 2.5 m high, in a sandpit about 11 m by 10.
    size: [1.5, 1.3], scale: 2.2,
    posts: [[-0.55, -0.42, 0.03], [-0.35, -0.42, 0.03], [-0.55, 0.12, 0.03], [-0.35, 0.12, 0.03], [0.22, -0.24, 0.08], [0.35, 0.32, 0.04]],
  },
];
const CIRCUIT_AMENITIES = [
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
];
const GALAXY_AMENITIES = [
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
  { // a dish listening to the sky
    head: merge([
      post(0, 0, 0.05, 0.2, '#2f2b45'), post(0, 0, 0.28, 0.04, '#4a4466', 0.05),
      painted(new THREE.SphereGeometry(0.34, 18, 6, 0, Math.PI * 2, Math.PI * 0.62, Math.PI * 0.38).rotateZ(0.7).translate(0.12, 0.5, 0), '#cfd2e6', 0.05),
      bar([0.05, 0.4, 0], [-0.18, 0.62, 0], 0.008, '#8b86a8'),
      painted(new THREE.IcosahedronGeometry(0.03, 0).translate(-0.18, 0.62, 0), '#6ff4ff'),
    ]),
    size: [0.8, 0.8], scale: 3,
    posts: [[0, 0, 0.2]],
    glow: { color: '#6ff4ff', shells: lit([[-0.18, 0.62, 0]], [[0.06, 0.45], [0.12, 0.15]]) },
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
];

// Each style's props, in the same four roles: a tall one for shores and parks, a low
// one beside it, and a light with its stem for the terrace edges.
// Implements: REQ-MAP-055, REQ-MAP-057
const PROPS = {
  city: {
    species: TREES, stem: '#5a4030', low: BUSH, lowHue: 0.25, amenities: CITY_AMENITIES,
    pole: POLE, poleColor: '#3a3d42', lampHead: HEAD, headColor: '#8a8d92', headNight: '#ffd28a',
    // Lit at night only, as street lamps are: a glow round the head, and its light
    // in a pool on the pavement.
    // Implements: REQ-CITY-036
    glowAt: 0.6, afterDark: true, pool: 0.34,
    // Tighter than an LED's: a lamp lights the street, it is not a ball of light.
    glow: [[0.045, 0.5], [0.08, 0.2], [0.13, 0.06]],
    solid: 0.055, post: 0.03,
    tint: hue => (it, c) => c.setHSL(hue + it.r * 0.07, 0.5 + 0.2 * it.r, 0.2 + it.r * 0.1),
  },
  circuit: {
    species: PARTS, stem: '#b9bec6', lows: SMD, amenities: CIRCUIT_AMENITIES,
    pole: LED_LEGS, poleColor: '#b9bec6', lampHead: LED, headColor: '#e2513c', headNight: '#ff6a52',
    // An LED is lit whether or not the room is: the glow sits over its lens.
    glowAt: 0.52,
    solid: 0.09, post: 0.03,
    // Parts are made in a handful of colors, not a spectrum: a little jitter around
    // the one the part type is usually sold in.
    tint: (hue, part) => (it, c) => c.setHSL(hue + (it.r - 0.5) * 0.04, hue < 0.05 ? 0.05 : 0.55, (part.light ?? 0.12) + it.r * 0.12),
  },
  galaxy: {
    species: CRYSTALS, stem: '#2b2540', lows: DEBRIS, amenities: GALAXY_AMENITIES,
    pole: BEACON_STEM, poleColor: '#2b2540', lampHead: BEACON, headColor: '#4fd0e8', headNight: '#9df0ff',
    // A beacon is lit day and night, as an LED is, and burns brighter in the dark.
    // Implements: REQ-MAP-057
    glowAt: 0.52, glow: [[0.075, 0.5], [0.13, 0.2], [0.22, 0.06]],
    solid: 0.085, post: 0.025,
    tint: hue => (it, c) => c.setHSL(hue + it.r * 0.12, 0.6 + 0.25 * it.r, 0.3 + it.r * 0.22),
  },
};

// The city's plants, and the galaxy's rubble, are models when props.js has them: the
// same trunk-and-crown shape as the blobs above, but a shape somebody drew. They are
// shaded here like everything else, so they take the map's flat paint and its
// per-instance tint and read as part of it rather than as an import.
//
// Built once, the first time makeProps runs after the models land.
let modeled = null;

const MODELS = ['tree1_stem', 'tree1_head', 'tree2_stem', 'tree2_head',
  'tree3_stem', 'tree3_head', 'bush_head', 'rock_head'];

function dressed(style) {
  const set = PROPS[style] || PROPS.city;
  const got = plants();
  // A file that loaded but is missing a part would be worse than no file at all.
  if (!got || MODELS.some(name => !got.has(name))) return set;
  modeled ||= {
    city: {
      ...PROPS.city,
      species: [1, 2, 3].map((n, i) => ({
        hue: PROPS.city.species[i].hue,
        stem: shaded(got.get(`tree${n}_stem`)),
        head: shaded(got.get(`tree${n}_head`), 0.16),
      })),
      low: shaded(got.get('bush_head'), 0.16),
    },
    galaxy: { ...PROPS.galaxy, lows: [{ ...DEBRIS[0], head: shaded(got.get('rock_head'), 0.12) }, ...DEBRIS.slice(1)] },
  };
  return modeled[style] || set;
}
