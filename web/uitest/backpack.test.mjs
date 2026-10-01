// The backpack as the page keeps it (backpack.js), and what it takes from the server.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { Backpack } = await import('../static/panels/backpack.js');

const item = (id, caughtAt, extra = {}) => ({ id, severity: 'high', title: id, where: '', line: 0, nodeId: '', caughtAt, fixed: false, fixedAt: 0, ...extra });

describe('backpack', () => {
  // Verifies: REQ-HUNT-028
  it('takes in what the server holds besides, newest first, keeping its own copy of the rest', () => {
    const quiet = [];
    const pack = new Backpack(`merge-${Math.random()}`, (_, q) => quiet.push(q));
    pack.replace([item('a', 30, { fixed: true }), item('b', 10)]);
    // The editor caught c while no map was open; the server also holds its own copy of a.
    assert.ok(pack.merge([item('c', 20), item('a', 30, { fixed: false })]));
    assert.deepEqual(pack.items.map(it => it.id), ['a', 'c', 'b']);
    assert.equal(pack.items[0].fixed, true, "the server's copy of an entry replaced the page's");
    assert.equal(quiet.at(-1), true, 'a merge is handed up by its caller, not by the redraw');
    assert.equal(pack.merge([item('b', 10)]), false, 'nothing new is no change');
  });
});
