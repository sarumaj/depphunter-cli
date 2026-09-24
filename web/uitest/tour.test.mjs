// The introduction a first visit is given.
//
// It is the one part of the UI that has to behave differently on a second visit than
// on a first, and the only way to see whether it does is to open it twice - which is
// exactly the thing nobody does by hand before shipping. The rest of what is checked
// here is that it cannot strand somebody on a card with no way off it, and that a
// browser refusing to remember anything does not turn it into a dialog on every load.

import assert from 'node:assert/strict';
import { describe, it, beforeEach } from 'node:test';

import './stub.mjs';

const { TOUR, WALK_TOUR, startTour, startWalkTour, walkTourPending } = await import('../static/tour.js');

const IDS = ['tour', 'tour-dots', 'tour-title', 'tour-body', 'tour-back', 'tour-next', 'tour-skip'];

/** Enough of the dialog for the introduction to drive, and a store it can write to. */
function page({ store = {} } = {}) {
  const make = () => ({
    children: [],
    classList: { on: false, toggle(_, v) { this.on = v; }, add() {}, remove() {} },
    style: {},
    replaceChildren(...c) { this.children = c; },
    addEventListener(name, fn) { this.closed = fn; },
    showModal() { this.open = true; },
    close() { this.open = false; this.closed?.(); },
  });
  const els = new Map(IDS.map(id => [id, make()]));
  globalThis.document = { getElementById: id => els.get(id) || null, createElement: make };
  globalThis.localStorage = {
    getItem: k => store[k] ?? null,
    setItem: (k, v) => { store[k] = String(v); },
  };
  return { els, store, at: () => els.get('tour-title').textContent };
}

describe('the introduction', () => {
  beforeEach(() => page());

  it('says something on every card, and never more than a full screen', () => {
    for (const deck of [TOUR, WALK_TOUR]) {
      assert.ok(deck.length >= 3 && deck.length <= 6, `${deck.length} cards is not an introduction`);
      for (const card of deck) {
        assert.ok(card.title.length > 0 && card.title.length < 40, `a poor title: ${card.title}`);
        const body = card.body.replace(/\s+/g, ' ').trim();
        assert.ok(body.length > 80, `${card.title} says almost nothing`);
        assert.ok(body.length < 460, `${card.title} is a page, not a card`);
      }
    }
  });

  it('keeps the two introductions apart', () => {
    const p = page();
    // Seeing the map's does not spend walk mode's, and the other way round: they are
    // two different things to be introduced to, at two different moments.
    startTour();
    for (let i = 0; i < TOUR.length; i++) p.els.get('tour-next').onclick();
    assert.equal(startTour(), false);
    assert.equal(startWalkTour(false), true, 'the map introduction spent walk mode\'s');
    for (let i = 0; i < WALK_TOUR.length; i++) p.els.get('tour-next').onclick();
    assert.equal(startWalkTour(false), false);
  });

  it('says in advance whether a walk needs explaining', () => {
    const p = page();
    // Walk mode asks before it takes the pointer: a dialog and a captured reticle at
    // the same time is a mouse fighting itself.
    assert.equal(walkTourPending(), true);
    startWalkTour(false);
    for (let i = 0; i < WALK_TOUR.length; i++) p.els.get('tour-next').onclick();
    assert.equal(walkTourPending(), false);
  });

  it('covers the keys somebody walking in has to know', () => {
    // As the dialog renders it: the cards are written wrapped, and a key can fall
    // across a line break in the source without doing so on screen.
    const said = WALK_TOUR.map(c => c.body.replace(/\s+/g, ' ')).join(' ');
    // Not every key, but nobody should have to guess at moving, at picking a tool up,
    // at using either hand, or at getting back out.
    for (const key of ['W, A, S and D', 'Shift', 'Space', 'Esc', '1 to 7', '8, 9 and 0',
      'T ', 'H ', 'click', 'F, C', 'Enter', 'B ', '?']) {
      assert.ok(said.includes(key), `walking in never mentions ${key.trim()}`);
    }
  });

  it('hands the pointer back once walk mode\'s is out of the way', () => {
    const p = page();
    let resumed = 0;
    startWalkTour(false, () => resumed++);
    assert.equal(resumed, 0, 'it let go before it was read');
    p.els.get('tour-skip').onclick();
    assert.equal(resumed, 1, 'a walker was left with no pointer and no reticle');
    // And only once, however many ways it was closed.
    p.els.get('tour').close();
    assert.equal(resumed, 1);
  });

  it('opens on a first visit and not on the next one', () => {
    const p = page();
    assert.equal(startTour(), true, 'a first visit was not shown anything');
    assert.equal(p.els.get('tour').open, true);
    assert.equal(p.at(), TOUR[0].title);

    // Walk to the end, which is what closes it and what remembers it.
    for (let i = 0; i < TOUR.length; i++) p.els.get('tour-next').onclick();
    assert.equal(p.els.get('tour').open, false, 'the last card did not close it');

    const again = page({ store: p.store });
    assert.equal(startTour(), false, 'it was shown twice');
    assert.notEqual(again.els.get('tour').open, true);
    // ... but the help can still ask for it.
    assert.equal(startTour(true), true, 'the help could not open it again');
  });

  it('goes back as well as forward, and stops at both ends', () => {
    const p = page();
    startTour();
    assert.equal(p.els.get('tour-back').disabled, true, 'the first card offered a way back');
    p.els.get('tour-next').onclick();
    assert.equal(p.at(), TOUR[1].title);
    assert.equal(p.els.get('tour-back').disabled, false);
    p.els.get('tour-back').onclick();
    assert.equal(p.at(), TOUR[0].title);
    p.els.get('tour-back').onclick(); // and no further
    assert.equal(p.at(), TOUR[0].title);
    // The last card offers to start rather than to go on.
    for (let i = 0; i < TOUR.length; i++) p.els.get('tour-next').onclick();
    assert.equal(p.els.get('tour-next').textContent, 'Start');
  });

  it('is skippable, and skipping counts as seen', () => {
    const p = page();
    startTour();
    p.els.get('tour-skip').onclick();
    assert.equal(p.els.get('tour').open, false);
    assert.equal(startTour(), false, 'skipping did not count');
  });

  it('stays out of the way when the browser will not remember anything', () => {
    page();
    globalThis.localStorage = {
      getItem() { throw new Error('blocked'); },
      setItem() { throw new Error('blocked'); },
    };
    // Unreadable is taken as seen: a dialog on every single load is worse than never
    // showing it to somebody who has blocked their storage.
    assert.equal(startTour(), false);
    // And asking for it from the help still works, and still does not throw.
    assert.equal(startTour(true), true);
  });
});
