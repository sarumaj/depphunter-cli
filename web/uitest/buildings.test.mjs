// The city's building types: which one a box is drawn as, and that none of them
// takes the box's color away from it.
//
// A type is chosen per box from a hash of its node's id and from its proportions,
// and the shaders average a far facade with the formula buildings.js facadeFar
// mirrors, so the colors checked here are the colors the map shows of a facade from
// the default view.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { box } from './stub.mjs';

const B = await import('../static/buildings.js');
const { CITY_FRAG_HEAD } = await import('../static/cityglsl.js');
const THREE = await import('../static/vendor/three.module.min.js');

// A synthetic district: every kind of box at every height the layout makes, in the
// sizes it makes them (files 1 x 1, symbols 0.42, collapsed directories broad).
function district(count = 3000) {
  const boxes = [];
  for (let i = 0; i < count; i++) {
    const h = 0.2 + (i % 103) / 102 * 10;
    boxes.push(box('building', i % 60, Math.floor(i / 60), 1, 1, { h, node: { id: `f:src/file${i}.go`, kind: 'file' } }));
  }
  for (let i = 0; i < 200; i++) {
    const side = 1.4 + (i % 7);
    boxes.push(box('district', i, -10, side, side, { h: 0.3 + (i % 11) * 0.5, node: { id: `d:directory${i}`, kind: 'dir' } }));
    boxes.push(box('symbol', i, -20, 0.42, 0.42, { h: [0.35, 0.7, 1.1][i % 3], node: { id: `s:symbol${i}`, kind: 'symbol' } }));
    boxes.push(box('package', i, -30, 1, 1, { h: 0.3 + (i % 9) * 0.6, node: { id: `p:package${i}`, kind: 'package' } }));
  }
  return boxes;
}

// The chroma of a linear color: its offset from the gray of the same mean.
const chroma = c => { const m = (c[0] + c[1] + c[2]) / 3; return c.map(v => v - m); };
const angle = (a, b) => {
  const dot = a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  const n = Math.hypot(...a) * Math.hypot(...b);
  return Math.acos(Math.max(-1, Math.min(1, dot / n))) * 180 / Math.PI;
};
const luminance = c => 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];

// Every color a building can be given in either theme: the language series, "other",
// packages and districts (style.css).
const PALETTE = [
  '#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#4a3aa7', '#a9a7a0', '#bdbab0', '#8f8b80',
  '#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#9085e9', '#6d6b66', '#55534d', '#7c786e', '#a07a2e',
];
const linear = hex => { const c = new THREE.Color(hex); return [c.r, c.g, c.b]; };

describe('building types', () => {
  // Verifies: REQ-CITY-031
  it('come from the node and the proportions only, the same on every run', () => {
    const boxes = district();
    const first = boxes.map(b => B.archetype(b));
    const again = district().map(b => B.archetype(b));
    assert.deepEqual(again, first);
    // Moved by a relayout, recolored, dimmed: the same building.
    for (const b of boxes.slice(0, 300)) {
      const moved = { ...b, x: b.x + 17.3, z: b.z - 4.1, color: '#ff0000', faded: true };
      assert.equal(B.archetype(moved), B.archetype(b), b.node.id);
      assert.deepEqual(B.buildParameters(moved), B.buildParameters(b));
    }
    // Land and terraces are not buildings.
    assert.equal(B.archetype(box('land', 0, 0, 9, 9)), -1);
    assert.equal(B.archetype(box('terrace', 0, 0, 9, 9)), -1);
  });

  // Verifies: REQ-CITY-031
  it('all occur on a large map', () => {
    const seen = new Map();
    for (const b of district()) {
      const t = B.archetype(b);
      seen.set(t, (seen.get(t) || 0) + 1);
    }
    for (const [i, name] of B.ARCHETYPES.entries()) assert.ok(seen.get(i) > 10, `${name}: ${seen.get(i) || 0}`);
  });

  // Verifies: REQ-CITY-031
  it('follow the proportions: low and broad is a warehouse, the tallest are towers', () => {
    const boxes = district();
    for (const b of boxes) {
      const t = B.archetype(b);
      if (t === B.TYPE.warehouse) assert.ok(b.h < B.LOW && Math.min(b.w, b.d) >= 0.9, `${b.node.id} h ${b.h}`);
      if (t === B.TYPE.deco) assert.ok(b.h >= B.DECO, `${b.node.id} h ${b.h}`);
      if (b.kind === 'symbol') assert.ok([B.TYPE.brick, B.TYPE.panel, B.TYPE.office].includes(t));
    }
    const tall = boxes.filter(b => b.kind === 'building' && b.h >= B.DECO).map(b => B.archetype(b));
    assert.ok(tall.filter(t => t === B.TYPE.deco || t === B.TYPE.office).length > tall.length * 0.7);
  });

  // Verifies: REQ-CITY-032
  it('keep the data color on the facade seen from afar, by day', () => {
    for (const hex of PALETTE) {
      const base = linear(hex);
      const colorful = Math.hypot(...chroma(base)) > 0.08;
      for (let type = 0; type < B.ARCHETYPES.length; type++) {
        for (const variant of [0, 0.5, 0.999]) {
          const far = B.facadeFar(base, type, variant, 0);
          const ratio = luminance(far) / luminance(base);
          assert.ok(ratio > 0.5 && ratio < (luminance(base) < 0.1 ? 1.45 : 1.3), `${hex} ${B.ARCHETYPES[type]}: brightness x${ratio.toFixed(2)}`);
          if (colorful) {
            const off = angle(chroma(far), chroma(base));
            assert.ok(off < 12, `${hex} ${B.ARCHETYPES[type]}: hue off by ${off.toFixed(1)} degrees`);
          }
        }
      }
    }
  });

  // Verifies: REQ-CITY-032
  it('keep the wall the larger part of every face, lit windows and all', () => {
    for (const name of B.ARCHETYPES) assert.ok(B.LOOKS[name].share <= 0.45 || B.LOOKS[name].tint >= 0.7, name);
    for (const hex of PALETTE) {
      const base = linear(hex);
      if (Math.hypot(...chroma(base)) <= 0.08) continue;
      for (let type = 0; type < B.ARCHETYPES.length; type++) {
        const off = angle(chroma(B.facadeFar(base, type, 0.5, 1)), chroma(base));
        assert.ok(off < 30, `${hex} ${B.ARCHETYPES[type]} at night: hue off by ${off.toFixed(1)} degrees`);
      }
    }
  });

  // Verifies: REQ-CITY-032
  it('are the table the shaders read', () => {
    assert.ok(CITY_FRAG_HEAD.includes(B.looksGLSL()), 'the shader was not built from the LOOKS table');
    for (const name of B.ARCHETYPES) assert.match(CITY_FRAG_HEAD, new RegExp(`const int ${name.toUpperCase()} = ${B.TYPE[name]};`));
  });
});
