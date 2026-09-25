// The photographs the camera keeps.
//
// A photograph is a megabyte of bitmap held by one object URL, so the two things worth
// checking are the two nobody notices going wrong: that a long session does not grow
// without bound, and that every picture let go of takes its URL with it. A leak here
// is invisible until a tab has been open for an hour.

import assert from 'node:assert/strict';
import { describe, it, beforeEach } from 'node:test';

import './stub.mjs';

const { Stash } = await import('../static/stash.js');

/** Counts the object URLs handed out and given back. */
function urls() {
  const live = new Set();
  let nth = 0;
  globalThis.URL.createObjectURL = () => {
    const u = `blob:${++nth}`;
    live.add(u);
    return u;
  };
  globalThis.URL.revokeObjectURL = u => live.delete(u);
  return live;
}

describe('the photograph stash', () => {
  let live;
  beforeEach(() => { live = urls(); });

  // Verifies: REQ-HUNT-034
  it('keeps what it is given, newest first', () => {
    const s = new Stash();
    s.add({}, 'one');
    const two = s.add({}, 'two');
    assert.equal(s.count, 2);
    assert.equal(s.items[0].id, two.id, 'the newest should be at the top');
    assert.equal(s.items[0].where, 'two');
    assert.equal(s.items[0].n, 2, 'photographs are numbered as they are taken');
    assert.equal(live.size, 2);
  });

  it('lets go of the oldest rather than refusing the newest', () => {
    const s = new Stash();
    for (let i = 0; i < 40; i++) s.add({}, `shot ${i}`);
    assert.ok(s.count < 40, 'it kept every one of forty photographs');
    assert.equal(s.items[0].where, 'shot 39', 'the one just taken was the one refused');
    assert.equal(live.size, s.count, `${live.size - s.count} bitmaps were left behind`);
  });

  it('frees a picture it no longer holds', () => {
    const s = new Stash();
    const a = s.add({}, 'a');
    s.add({}, 'b');
    s.remove(a.id);
    assert.equal(s.count, 1);
    assert.equal(live.size, 1, 'removing one did not free it');
    s.remove('nothing at all'); // and an id it does not have is not an error
    assert.equal(s.count, 1);
    s.clear();
    assert.equal(s.count, 0);
    assert.equal(live.size, 0, 'emptying it left bitmaps behind');
  });

  it('says when it has changed, so whatever draws it can', () => {
    let drawn = 0;
    const s = new Stash(() => drawn++);
    const a = s.add({}, 'a');
    s.remove(a.id);
    s.clear();
    assert.equal(drawn, 3);
  });

  // Verifies: REQ-HUNT-035
  it('names a saved file after the repository and the photograph', () => {
    const clicked = [];
    globalThis.document.createElement = () => ({ click() { clicked.push(this.download); } });
    const s = new Stash();
    const it = s.add({}, 'a wall');
    assert.equal(s.save(it.id, 'my-repo'), 'my-repo-photo-01.png');
    assert.deepEqual(clicked, ['my-repo-photo-01.png']);
    assert.equal(s.save('nothing at all', 'my-repo'), null);
  });
});
