// How tall a collapsed directory is drawn.
//
// A district stands in for the files under it, so it is as tall as the average of
// them would be - on the very scale their buildings use. The layout reads that average
// off what computeVisibility counted for the directory, which is why the two are
// tested together here: the layout's arithmetic was right all along, and it was the
// count that stopped carrying the drawn size.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { buildModel } = await import('../static/model.js');
const { computeVisibility } = await import('../static/filter.js');
const { layout } = await import('../static/layout.js');

/** A file node as the graph document carries one. */
const file = (name, parent, extra) =>
  ({ id: `f:${name}`, kind: 'file', name, path: name, parent, lang: 'go', ...extra });
const dir = (name, parent) => ({ id: `d:${name}`, kind: 'dir', name, path: name, parent });

// One file of 500 lines alone in src/, a pair averaging 1500 in big/, and a 2000-line
// file at the top so that neither is the tallest thing on the map.
const model = buildModel({
  nodes: [
    { id: 'd:.', kind: 'dir', name: '.', path: '.' },
    dir('src', 'd:.'), dir('big', 'd:.'),
    file('src/a.go', 'd:src', { loc: 500 }),
    file('big/b.go', 'd:big', { loc: 1000 }), file('big/c.go', 'd:big', { loc: 2000 }),
    file('top.go', 'd:.', { loc: 2000 }),
  ],
  edges: [],
});

const filters = { hiddenLangs: new Set(), hiddenEcosystems: new Set(), path: '' };

/** The box drawn for a node with these directories expanded. */
const drawn = (id, ...expanded) => layout(model, {
  vis: computeVisibility(model, filters),
  expanded: new Set(['d:.', ...expanded]),
  heightScale: 'sqrt',
}).byNode.get(id);

describe('collapsed directory height', () => {
  // Verifies: REQ-MAP-004
  it('is as tall as a building for its mean file', () => {
    const district = drawn('d:src');
    const building = drawn('f:src/a.go', 'd:src');
    assert.equal(district.kind, 'district');
    assert.equal(building.kind, 'building');
    assert.ok(district.h > 0.2, 'the district was drawn at the floor height');
    assert.ok(Math.abs(district.h - building.h) < 1e-9,
      `district ${district.h} and its 500-line building ${building.h} differ`);
  });

  // Verifies: REQ-MAP-004
  it('grows with the mean size of the files beneath it', () => {
    assert.ok(drawn('d:big').h > drawn('d:src').h);
  });

  // Verifies: REQ-MAP-004
  it('counts only the files a filter leaves standing', () => {
    // Hiding the 2000-line file in big/ leaves its 1000-line one, so the district
    // comes down to that file's height rather than keeping the average of both.
    const vis = computeVisibility(model, { ...filters, path: '!big/c.go' });
    assert.equal(vis.counts.get('d:big').totalBulk, 1000);
    assert.equal(vis.counts.get('d:big').fileCount, 1);
  });
});
