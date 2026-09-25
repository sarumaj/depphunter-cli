// How big a thing is, which the map answers twice and must not confuse.
//
// A file arrives with lines counted, or - if it is binary, or over --max-file-size -
// with nothing counted and a size in bytes. Everything the map draws needs an answer
// for both, and everything the map says in words needs an answer only for the first:
// a stand-in in a stated number is a lie about the file, where a stand-in in a
// building's height is only a building.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { buildModel, bulk, unread, fileSize } = await import('../static/model.js');

/** A file node as the graph document carries one. */
const file = (name, extra) => ({ id: `f:${name}`, kind: 'file', name, path: name, parent: 'd:.', ...extra });

const model = (...files) => buildModel({
  nodes: [{ id: 'd:.', kind: 'dir', name: '.', path: '.' }, ...files],
  edges: [],
});

describe('how big a file is', () => {
  // Verifies: REQ-MAP-058
  it('measures a read file in its own lines', () => {
    assert.equal(bulk(file('a.go', { loc: 500, bytes: 18000 })), 500);
    assert.equal(unread(file('a.go', { loc: 500, bytes: 18000 })), false);
  });

  // Verifies: REQ-MAP-058, REQ-MAP-060
  it('falls back to bytes for a file nothing read', () => {
    // The case this exists for: a model, an image, anything over the size limit. It
    // has to come out as a building with a storey on it rather than the bare floor
    // every one of them shared while lines were the only measure.
    const blob = file('bug.glb', { bytes: 4_200_000 });
    assert.equal(unread(blob), true);
    assert.ok(bulk(blob) > 0, 'a 4 MB file still measures nothing');
    // ... and proportionally: twice the bytes is twice the building.
    assert.ok(bulk(file('b', { bytes: 2000 })) > bulk(file('a', { bytes: 1000 })));
  });

  it('says nothing about a file it has nothing to say about', () => {
    // No lines and no bytes is a file that was never measured at all, and zero is the
    // honest answer - not the floor of some scale.
    assert.equal(bulk(file('gone', {})), 0);
    assert.equal(unread(file('gone', {})), false, 'an unmeasured file claims a size');
    assert.equal(unread({ kind: 'dir', name: 'src' }), false);
  });

  // Verifies: REQ-MAP-059
  it('keeps stated lines and drawn size apart', () => {
    // The one rule. totalLoc is what the status bar and the panel put in words, so it
    // counts only lines somebody counted; totalBulk is what the geometry asks for, so
    // it counts the blob too. A binary raising the stated line count of a directory
    // would be the map telling somebody their repository is bigger than it is.
    const m = model(file('a.go', { loc: 300, bytes: 11000 }), file('bug.glb', { bytes: 4_000_000 }));
    const root = m.byId.get('d:.');
    assert.equal(root.totalLoc, 300, 'a binary was counted as lines of source');
    assert.ok(root.totalBulk > root.totalLoc, 'a binary weighed nothing in the geometry');
    assert.equal(root.fileCount, 2);
  });

  // Verifies: REQ-MAP-060
  it('writes a size somebody can read at a glance', () => {
    assert.equal(fileSize(0), '0 bytes');
    assert.equal(fileSize(96), '96 bytes');
    assert.equal(fileSize(812_000), '812 kB');
    assert.equal(fileSize(4_200_000), '4.2 MB');
    // Ten and over loses the decimal, which nobody was reading anyway.
    assert.equal(fileSize(42_000_000), '42 MB');
    assert.equal(fileSize(3_000_000_000), '3.0 GB');
  });
});
