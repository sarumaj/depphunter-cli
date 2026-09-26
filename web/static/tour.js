// The introduction: what a first visit is told about the map before it is looked at.
//
// A map of a repository is not a thing anyone has seen before, so the shapes have to be
// said out loud once. This says the three that nothing on screen can say for itself -
// what the buildings and the islands stand for, that the map is also a place you can
// walk into, and that the findings are out there to be collected - and then stops.
// Everything else is in the help, which is where somebody who wants the whole of it
// will look.
//
// There are two of them, because there are two things to be introduced to and no
// reason to explain the second before anybody has asked for it: the map gets its cards
// on a first load, and walk mode gets its own the first time somebody walks into it.
//
// Both are remembered per browser rather than per repository, because what they explain
// is the tool and not the project.

import { $ } from './dom.js';
import { STATIC } from './data.js';

/** The map's cards, in order. */
// Implements: REQ-UI-004
export const TOUR = [
  {
    shot: 'map',
    alt: 'The map: a mainland of terraced buildings with dependency arcs over it, and package islands around it.',
    title: 'This is your repository',
    body: `The mainland is the project. Directories are terraces, the files standing on
      them are buildings, and a building's height is the number of lines in the file.
      The islands around it are the package ecosystems the project depends on, the most
      depended-upon nearest the shore. Color is the language, unless you ask the color
      menu for something else.`,
  },
  {
    title: 'Selecting draws the dependencies',
    body: `Click a building and the map shows what it depends on and what depends on it:
      an arc overhead to each, with an arrow at the far end for which way it runs, and
      everything else fades so the arcs stand out. The panel at the side says what it
      is and lists both directions in full. Double-click a directory to open it, or a
      file to see its functions as plots on a terrace.`,
  },
  {
    shot: 'street',
    alt: 'Walk mode: the same map seen from a street inside it, with a tool held in each hand.',
    title: 'The map is also a place',
    body: `Press V to walk into it. You explore in first person on a small planet,
      holding a tool in each hand: one for the hunt, one to carry you — a grapple line,
      a jet backpack, a pair of floats for the water. Using a tool on a building selects
      that module and marks it with a beacon, so the walk and the map view are the same
      session seen two ways.`,
  },
  {
    title: 'The findings are out there',
    body: `Point depphunter at the reports your scanners already produce and every
      finding becomes a bug patrolling the building it belongs to — a caterpillar for a
      critical one, a beetle for the middle of the range, a mite for a note. Catch one
      and it tells you what was reported, and goes into your backpack until the source
      is fixed. They bite, so watch your health.`,
  },
  {
    title: 'Press ? at any time',
    body: `The help lists every key, every tool and what each one is for. The toolbar
      above changes what color and height mean, filters languages and ecosystems, and
      exports the map — as an image, as a graph, or as one self-contained page you can
      send to somebody who has none of this installed.`,
  },
];

/**
 * Walk mode's cards, shown the first time somebody walks into the map. It is a
 * different thing to learn - a pair of hands and a body rather than a diagram - and
 * saying it on the way in is worth more than saying it on a first load, where there is
 * nothing yet to try it on.
 * Implements: REQ-UI-011
 */
export const WALK_TOUR = [
  {
    title: 'Getting about',
    body: `W, A, S and D walk, the arrow keys walk and turn, Shift runs and Space
      jumps — the last two spend your wind, the second gauge in the corner, which fills
      again while you walk or stand still. The mouse looks around: it is captured at
      the reticle, and Esc gives it back. The world is bent around a
      small planet, so the streets fall away over the horizon; press + or − if you
      would rather they did that more or less. V, M or Esc returns you to the map.`,
  },
  {
    shot: 'wheel',
    alt: 'The tool wheel open, the hunt down its right side and what carries you down its left.',
    title: 'A tool in each hand',
    body: `Hold R for the tool wheel: the hunt down its right side — rod, net,
      camera, bubble wand, extinguisher, dart, nail gun — and down its left what
      carries you: a grapple gun, a jet backpack, skimmers, or an empty hand. Point
      at one and let go; you are held still while it is up. Without looking: E is the
      next tool for your right hand, Q the next for your left, and 1 to 0 pick along
      the row at the bottom. H stows both hands.`,
  },
  {
    title: 'Using them',
    body: `A click uses your right hand; hold the button and the nail gun and the
      extinguisher keep firing. F, C or the middle mouse button uses your left. Holding
      the right mouse button looks through the scope, which is what the tracking dart
      is for, and the wheel zooms. Each tool has its own range, and the reticle dims
      when what you are pointing at is out of it.`,
  },
  {
    title: 'What using one does',
    body: `On a building, the rod, the camera, the dart or the nail gun tags the
      module: it is selected, its dependency trails light up, and a beacon is planted
      over it in the color of the worst thing the scanners found inside. Use the tool on it again, or press Enter, to read the
      details without leaving the street. O opens the file in your editor.`,
  },
  {
    shot: 'tracker',
    alt: 'The tracker: a sweep of the streets around you, with what is still out there marked on it.',
    title: 'The bugs, and the backpack',
    body: `Every finding walks a lap on the building it belongs to — a caterpillar for
      a critical one, a beetle for the middle of the range, a mite for a note. Catch
      one and it tells you what was reported and goes into your backpack, which B opens
      from here. The sweep in the corner shows what is still out there and tightens as
      you close on it.`,
  },
  {
    shot: 'fire',
    alt: 'A roof alight: a vulnerability something in the project actually calls, burning where it was reached.',
    title: 'Fire, and staying in one piece',
    body: `A vulnerability your code can actually reach burns where it was reached, and
      spreads along the calls. Put it out with the extinguisher — douse the package it
      came from and the whole chain goes out, which is what upgrading it does. Standing
      in one hurts, as do bugs, long falls and deep water with nothing to float on. Get
      clear and you mend. Press ? for everything, at any time.`,
  },
];

// Implements: REQ-UI-005, REQ-UI-012
const TOUR_SEEN = 'depphunter.introduced';
const WALK_SEEN = 'depphunter.introduced.walk';

/**
 * Opens the map's introduction: once on a first visit, and whenever the help asks for
 * it again, which is what `forced` is for.
 * Implements: REQ-UI-004
 */
export const startTour = (forced = false) => show(TOUR, TOUR_SEEN, forced);

/**
 * Walk mode's, the first time somebody walks in. `done` is called once it is out of the
 * way, which is how walk mode knows to take the pointer back: the dialog has it while
 * it is open, and a walker with no pointer and no reticle is stuck.
 * Implements: REQ-UI-011, REQ-UI-013
 */
export const startWalkTour = (forced, done) => show(WALK_TOUR, WALK_SEEN, forced, done);

/**
 * Whether walking in would show it. The caller needs to know before the walker is in
 * the street, because a first walk is explained before the pointer is taken rather
 * than after: asking for the reticle and giving it straight back leaves the mouse
 * fighting the dialog for the same few seconds.
 * Implements: REQ-UI-013
 */
export const walkTourPending = () => !remembered(WALK_SEEN);

/** Whether a deck has been seen. A store that cannot be read counts as seen. */
// Implements: REQ-UI-005, REQ-UI-007
function remembered(key) {
  try {
    return localStorage.getItem(key) === '1';
  } catch {
    return true; // a blocked store is not a reason to show it on every load
  }
}

/**
 * One deck of cards, if it has not been seen. Returns whether it was opened, so a
 * caller can tell the difference between "shown" and "already seen"; `after` runs when
 * it closes, however it was closed.
 * Implements: REQ-UI-004, REQ-UI-006
 */
function show(cards, key, forced = false, after = null) {
  if (remembered(key) && !forced) return false;
  const dialog = $('tour'), dots = $('tour-dots');
  dots.replaceChildren(...cards.map(() => document.createElement('i')));
  let at = 0;
  const draw = () => {
    const card = cards[at];
    // The picture, where the card has one. They are taken off the running map by
    // scripts/tour-shots.mjs rather than drawn, so that they cannot quietly stop being
    // true: a drawing of a feature is a thing somebody has to remember to redraw.
    //
    // A missing one is not an error. The pictures are generated, and a working copy
    // that has not generated them yet - or an export made before they existed - shows
    // the cards as they always were, in words. Which is also why nothing here waits
    // for them: the introduction opens on the first load of the map, and a card that
    // held still until an image decoded would be a card that arrives late.
    const shot = $('tour-shot');
    shot.hidden = !card.shot;
    if (card.shot) {
      // An exported map has no server to fetch from and carries the pictures inline,
      // the same way it carries the models (web/static.go).
      shot.src = STATIC?.tour?.[card.shot] || `tour/${card.shot}.webp`;
      shot.alt = card.alt || '';
      shot.onerror = () => { shot.hidden = true; };
    }
    $('tour-title').textContent = card.title;
    $('tour-body').textContent = card.body.replace(/\s+/g, ' ').trim();
    $('tour-back').disabled = at === 0;
    $('tour-next').textContent = at === cards.length - 1 ? 'Start' : 'Next';
    [...dots.children].forEach((d, i) => d.classList.toggle('on', i === at));
  };
  // Three things end it - the last card, Skip, and Esc - and the first two close the
  // dialog, which is itself one of the three. So it is written down once and run once,
  // whichever way round it was reached.
  let ended = false;
  const done = () => {
    if (ended) return;
    ended = true;
    try {
      localStorage.setItem(key, '1');
    } catch {
      // Nothing to remember it with; it will be offered again, which is no worse
      // than a tool that cannot save its settings either.
    }
    dialog.close();
    after?.();
  };
  $('tour-back').onclick = () => { at = Math.max(0, at - 1); draw(); };
  $('tour-next').onclick = () => {
    if (at === cards.length - 1) return done();
    at++;
    draw();
  };
  $('tour-skip').onclick = done;
  dialog.addEventListener('close', done, { once: true });
  draw();
  dialog.showModal();
  return true;
}
