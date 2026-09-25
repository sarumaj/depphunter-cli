// Opening a menu from a captured pointer.
//
// Letting go of a pointer lock takes a moment, and in that moment the canvas still
// has the pointer: a menu shown straight away could be shut by a click-outside
// handler reading an event meant for the street. whenUnlocked holds the opening back
// until the browser says the lock is gone. The browser is a stand-in here - a lock
// that is let go of only when the test says so.

import assert from 'node:assert/strict';
import { describe, it, beforeEach, mock } from 'node:test';

import './stub.mjs';

const { whenUnlocked } = await import('../static/dom.js');

/** A document with a pointer lock the test releases by hand. */
function locked(held = true) {
  const listeners = new Set();
  const doc = {
    pointerLockElement: held ? {} : null,
    exits: 0,
    exitPointerLock() { this.exits++; },
    addEventListener(name, fn) { if (name === 'pointerlockchange') listeners.add(fn); },
    removeEventListener(name, fn) { if (name === 'pointerlockchange') listeners.delete(fn); },
    /** The browser reporting a change, with the lock now `still` held or not. */
    change(still = false) {
      this.pointerLockElement = still ? {} : null;
      for (const fn of [...listeners]) fn();
    },
    listeners,
  };
  globalThis.document = doc;
  return doc;
}

describe('a menu that wants the pointer', () => {
  beforeEach(() => mock.timers.enable({ apis: ['setTimeout'] }));

  // Verifies: REQ-UI-014
  it('opens at once when nothing holds the pointer', () => {
    const doc = locked(false);
    let opened = 0;
    whenUnlocked(() => opened++);
    assert.equal(opened, 1);
    assert.equal(doc.exits, 0, 'let go of a lock nobody held');
    mock.timers.reset();
  });

  // Verifies: REQ-UI-014
  it('waits for the lock to be gone before opening', () => {
    const doc = locked();
    let opened = 0;
    whenUnlocked(() => opened++);
    assert.equal(doc.exits, 1, 'the lock was never let go of');
    assert.equal(opened, 0, 'opened while the canvas still had the pointer');
    // A change that leaves the lock held (the canvas taking it again) is not it.
    doc.change(true);
    assert.equal(opened, 0);
    doc.change(false);
    assert.equal(opened, 1);
    // ... and once only, whatever else is reported or runs out afterwards.
    doc.change(false);
    mock.timers.tick(1000);
    assert.equal(opened, 1);
    assert.equal(doc.listeners.size, 0, 'left a listener behind');
    mock.timers.reset();
  });

  // Verifies: REQ-UI-014
  it('opens anyway when the release is never reported', () => {
    locked();
    let opened = 0;
    whenUnlocked(() => opened++, 250);
    mock.timers.tick(249);
    assert.equal(opened, 0);
    mock.timers.tick(1);
    assert.equal(opened, 1);
    mock.timers.reset();
  });

  // Verifies: REQ-UI-014
  it('can be called off while it waits', () => {
    const doc = locked();
    let opened = 0;
    const cancel = whenUnlocked(() => opened++);
    cancel();
    doc.change(false);
    mock.timers.tick(1000);
    assert.equal(opened, 0, 'a menu closed while waiting opened anyway');
    mock.timers.reset();
  });
});
