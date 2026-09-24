// How a walker changes what is in their hands.
//
// Walk mode carries two tools at once - one in each hand - out of a bag of ten, and
// the row along the bottom of the HUD is ten slots wide. A row that wide is read by
// counting along it, and counting along it is the one thing nobody does in the middle
// of a chase. So there are three ways in, and they are meant for three different
// moments:
//
//   E   the next tool for the right hand, which is the hunt. It cycles and never
//       leaves the hand empty: there is always something to hunt with.
//   Q   the next tool for the left hand, which is what carries you - and, at the end
//       of that ring, nothing at all. Putting the jet backpack away is how you come
//       down and stepping off the skimmers is how you go in the water, so an empty
//       hand is a choice like any other and belongs in the ring with the rest.
//   R   the wheel: every tool at once, laid out where its hand is, picked by pointing
//       at it. Held down it is a flick and a release; tapped it stays up to be read.
//
// The wheel is laid out the way the walker is, which is the whole reason it is a
// wheel rather than a list. The right half is the hunt's row, top to bottom; the left
// half is what the off hand carries, and the bare hand sits at the bottom of it. That
// is the same arrangement as the slot row - left hand on the left, right hand on the
// right - except that here the two halves are literally two sides rather than two
// groups in a line, so nothing has to be numbered to say which is which.
//
// And the digits are still there under all of it - but numbered along the row this
// time. They used to be assigned by kind: 1 upwards for the hunt and 8, 9 and 0 for
// the carried tools, which are drawn at the left-hand end because that is the hand
// they go in. So the row read 8, 9, 0, 1, 2 ... from left to right: its keys and its
// layout disagreed about which came first, and a walker counting along it to find
// the fourth slot found a 1.
//
// Now there is one order and both follow it. `rowOrder` below is the list the HUD
// lays out and the list the digits count along, so the row reads 1 to 0 from left to
// right and a tool's key is wherever it is drawn. Changing the layout renumbers the
// keys with it, which is the only arrangement in which the two cannot drift apart.

import { PRIMARY_IDS, SECONDARY_IDS, toolFor } from './tools.js';

/** The left hand holding nothing, which is a choice on the wheel like any tool. */
export const EMPTY = 'none';

/** What the off hand cycles through: everything it can carry, and then nothing. */
export const carriedRing = () => [...SECONDARY_IDS, EMPTY];

/**
 * One step along a ring of tools. `current` may be anything not on the ring - an
 * empty hand where EMPTY is not a member of it - and the step then lands on the
 * first, which is what makes the first press of a cycling key predictable.
 */
export function cycle(ring, current, dir = 1) {
  const at = ring.indexOf(current);
  return at < 0 ? ring[dir > 0 ? 0 : ring.length - 1] : ring[(at + dir + ring.length) % ring.length];
}

/**
 * The tools in the order they are laid out and numbered: what the left hand carries
 * first, because that is the hand it goes in and the left of the screen is where its
 * tool is drawn, and then the hunt's row. The HUD builds its slots from this and the
 * digits count along it, so there is one order rather than two.
 */
export const rowOrder = () => [...SECONDARY_IDS, ...PRIMARY_IDS];

/**
 * The digit a slot wears: its place in the row, 1 upwards, with the tenth on 0 the
 * way a shooter numbers a tenth slot. Ten slots is exactly ten digits, which is the
 * only reason this can be as simple as counting.
 */
export const keyFor = id => {
  const at = rowOrder().indexOf(id);
  return at < 0 ? '' : String((at + 1) % 10);
};

/**
 * The tool a number key picks, or undefined for every key that is not one.
 *
 * The guard is the whole of it: this is asked about every key the walker presses, and
 * `'KeyQ'.slice(5)` is an empty string, which +coerces to nought - so without it, Q
 * would come through here as the 0 key and put a nail gun in the hunting hand instead
 * of walking the carried row.
 */
export const toolForKey = code =>
  /^Digit[0-9]$/.test(code) ? rowOrder()[(+code.slice(5) + 9) % 10] : undefined;

/**
 * Every key that picks a tool directly, for the help and for what a slot says: its
 * digit, and Q as well for the tools the off hand carries, since Q walks that row
 * without a hand leaving W, A, S and D.
 */
export const keysFor = id =>
  SECONDARY_IDS.includes(id) ? [keyFor(id), 'Q'] : [keyFor(id)];

// The wheel, in pixels: how big its face is, how far out its tools sit, and how wide
// the hub in the middle is. The hub is the dead zone as well as the label - a cursor
// that has not left the middle is pointing at nothing, so opening the wheel and
// letting it go without moving changes no hands at all.
export const FACE = 320, RING = 112, HUB = 58;

/**
 * Every wedge of the wheel: what it holds and the angles it spans, measured from the
 * top and running clockwise the way a conic gradient does.
 *
 * The two halves are sized by what is in them rather than cut to a common angle: the
 * hunt's seven share the right half and the carried three and the empty hand share
 * the left. So the wedges differ in width between the halves and the halves are what
 * the eye reads, which is the point.
 */
export function wedges() {
  const left = carriedRing();
  const rs = 180 / PRIMARY_IDS.length, ls = 180 / left.length;
  return [
    ...PRIMARY_IDS.map((id, i) => ({ id, from: i * rs, to: (i + 1) * rs })),
    // Counted up from the top the other way, so the first carried tool is across from
    // the first of the hunt's and the bare hand ends up at the bottom.
    ...left.map((id, i) => ({ id, from: 360 - (i + 1) * ls, to: 360 - i * ls })),
  ];
}

/** Where the middle of a wedge is, as an offset from the middle of the wheel. */
export function seatOf(w, r = RING) {
  const a = ((w.from + w.to) / 2 - 90) * Math.PI / 180;
  return { x: Math.cos(a) * r, y: Math.sin(a) * r };
}

/**
 * The wedge a cursor at (dx, dy) from the middle is on, or null for the hub - which
 * is not a refusal to choose so much as the choice to keep what is already in hand.
 */
export function wedgeAt(dx, dy) {
  if (Math.hypot(dx, dy) < HUB) return null;
  const a = (Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360;
  return wedges().find(w => a >= w.from && a < w.to) || null;
}

/** The name and the line under it that the hub shows for a wedge. */
function saysOf(id) {
  if (id !== EMPTY) return { name: toolFor(id).label, hint: toolFor(id).hint };
  return { name: 'Bare left hand', hint: 'Put down what you are carrying - which is how you come down out of the air' };
}

/**
 * The wheel itself. It is built once, on the first open, and then only moved: a
 * wheel rebuilt every time it comes up would drop the browser's work on eleven
 * gradients and eleven labels into the middle of whatever the walker was doing.
 *
 * It draws nothing about the world and decides nothing about the walker. What it is
 * pointing at is `pick`, and who reads it and what they do about it is walk.js's
 * business.
 */
export class ToolWheel {
  constructor(el) {
    this.el = el;
    this.x = 0;
    this.y = 0;
    this.at = null;   // the wedge under the cursor, or null for the hub
    this.built = false;
  }

  /** What the wheel is pointing at: a tool's id, EMPTY, or null for nothing. */
  get pick() { return this.at?.id ?? null; }

  build() {
    if (this.built) return;
    this.built = true;
    const face = document.createElement('div');
    face.className = 'w-wheel-face';
    face.style.setProperty('--face', `${FACE}px`);
    face.style.setProperty('--hub', `${HUB}px`);

    const lit = document.createElement('div');
    lit.className = 'w-wheel-lit';
    face.append(lit);

    this.seats = new Map();
    for (const w of wedges()) {
      const split = document.createElement('div');
      split.className = 'w-wheel-split';
      split.style.transform = `rotate(${w.from + 180}deg)`;
      face.append(split);

      // The same markup a slot in the row is built from, so a tool wears the same
      // shape in both places without either one having to describe it twice.
      const tool = w.id === EMPTY ? null : toolFor(w.id);
      const seat = document.createElement('div');
      seat.className = 'w-slot';
      seat.dataset.tool = w.id;
      seat.dataset.kind = tool ? tool.kind : 'secondary';
      seat.setAttribute('role', 'option');
      const { x, y } = seatOf(w);
      seat.style.transform = `translate(-50%, -50%) translate(${x}px, ${y}px)`;
      seat.append(
        Object.assign(document.createElement('kbd'), { textContent: keyFor(w.id) }),
        document.createElement('i'),
      );
      seat.setAttribute('aria-label', saysOf(w.id).name);
      face.append(seat);
      this.seats.set(w.id, seat);
    }

    this.hub = document.createElement('div');
    this.hub.className = 'w-wheel-hub';
    this.name = document.createElement('b');
    this.says = document.createElement('span');
    this.hub.append(this.name, this.says);
    face.append(this.hub);

    this.dot = document.createElement('div');
    this.dot.className = 'w-wheel-dot';
    face.append(this.dot);

    this.el.replaceChildren(face);
    this.lit = lit;
  }

  /** Bring it up, with the cursor in the middle so that letting go changes nothing. */
  raise(held) {
    this.build();
    this.x = this.y = 0;
    this.at = null;
    this.el.hidden = false;
    this.draw(held);
  }

  close() {
    this.el.hidden = true;
    this.at = null;
  }

  /** Whether it is up. The element's own hidden flag is the state; there is no other. */
  get open() { return !this.el.hidden; }

  /**
   * Move the cursor. What arrives is the movement rather than a point: walk mode
   * captures the pointer at the reticle, so there is no page coordinate to read, and
   * where the capture is refused the wheel is still aimed by how far the mouse went.
   */
  move(dx, dy, held) {
    this.x += dx;
    this.y += dy;
    // Past the rim the cursor stops rather than sails off: an arm's worth of mouse in
    // one direction should still be pointing at the wedge it arrived at.
    const r = Math.hypot(this.x, this.y), max = FACE / 2 - 6;
    if (r > max) { this.x *= max / r; this.y *= max / r; }
    this.at = wedgeAt(this.x, this.y);
    this.draw(held);
  }

  /**
   * What is in hand, so the wheel can mark it and the hub can name it while the
   * cursor is on nothing: {primary, secondary, dry}.
   */
  draw(held = {}) {
    if (!this.built) return;
    this.dot.style.transform = `translate(-50%, -50%) translate(${this.x}px, ${this.y}px)`;
    const on = this.at;
    this.lit.style.setProperty('--from', `${on ? on.from : 0}deg`);
    this.lit.style.setProperty('--span', `${on ? on.to - on.from : 0}deg`);
    for (const [id, seat] of this.seats) {
      const out = id === held.primary || id === held.secondary
        || (id === EMPTY && !held.secondary);
      seat.setAttribute('aria-selected', String(out));
      seat.classList.toggle('on', id === on?.id);
      // A tool with an empty tank is still on the wheel and still takes, because it
      // fills while it is not in hand and reaching for it is how you find out.
      seat.classList.toggle('dry', !!held.dry?.has(id));
    }
    if (on) {
      const { name, hint } = saysOf(on.id);
      this.name.textContent = name;
      this.says.textContent = hint;
    } else {
      this.name.textContent = 'Keep what you are holding';
      this.says.textContent = 'Point at a tool: the hunt on the right, what carries you on the left';
    }
  }
}
