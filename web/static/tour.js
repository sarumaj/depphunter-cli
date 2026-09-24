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

/** The map's cards, in order. */
export const TOUR = [
  {
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
      arcs overhead for the shape of it, and roads through the streets for the route,
      with chevrons marking which way each dependency runs. Double-click a directory to
      open it, or a file to see its functions as plots on a terrace.`,
  },
  {
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
    title: 'A tool in each hand',
    body: `Keys 1 to 7 put a tool in your right hand: a fishing rod, a butterfly net, a
      camera, a bubble wand, a fire extinguisher, a tracking dart or a nail gun. T
      walks along that row. Keys 8, 9 and 0 pick up something for your left hand — a
      grapple gun, a jet backpack or a pair of water skimmers — and pressing the same
      key again puts it down. H stows both hands without putting either away.`,
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
    body: `On a building, any of them tags the module: it is selected, its dependency
      trails light up, and a beacon is planted over it in the color of the worst thing
      the scanners found inside. Use the tool on it again, or press Enter, to read the
      details without leaving the street. O opens the file in your editor.`,
  },
  {
    title: 'The bugs, and the backpack',
    body: `Every finding walks a lap on the building it belongs to — a caterpillar for
      a critical one, a beetle for the middle of the range, a mite for a note. Catch
      one and it tells you what was reported and goes into your backpack, which B opens
      from here. The sweep in the corner shows what is still out there and tightens as
      you close on it.`,
  },
  {
    title: 'Staying in one piece',
    body: `The bar in the corner is what you can take: bugs bite, long falls hurt, and
      deep water with nothing to float on drowns you — and the bay is a step down from
      any shore, so walking into it is as easy as meaning to. Every bug in your backpack
      raises it. The jet backpack and the skimmers run on a tank that fills again while they
      are not in use — the gauge beside your health is what is left. Press ? for
      everything, at any time.`,
  },
];

const TOUR_SEEN = 'depphunter.introduced';
const WALK_SEEN = 'depphunter.introduced.walk';

/**
 * Opens the map's introduction: once on a first visit, and whenever the help asks for
 * it again, which is what `forced` is for.
 */
export const startTour = (forced = false) => show(TOUR, TOUR_SEEN, forced);

/**
 * Walk mode's, the first time somebody walks in. `done` is called once it is out of the
 * way, which is how walk mode knows to take the pointer back: the dialog has it while
 * it is open, and a walker with no pointer and no reticle is stuck.
 */
export const startWalkTour = (forced, done) => show(WALK_TOUR, WALK_SEEN, forced, done);

/**
 * Whether walking in would show it. The caller needs to know before the walker is in
 * the street, because a first walk is explained before the pointer is taken rather
 * than after: asking for the reticle and giving it straight back leaves the mouse
 * fighting the dialog for the same few seconds.
 */
export const walkTourPending = () => !remembered(WALK_SEEN);

/** Whether a deck has been seen. A store that cannot be read counts as seen. */
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
 */
function show(cards, key, forced = false, after = null) {
  if (remembered(key) && !forced) return false;
  const dialog = $('tour'), dots = $('tour-dots');
  dots.replaceChildren(...cards.map(() => document.createElement('i')));
  let at = 0;
  const draw = () => {
    $('tour-title').textContent = cards[at].title;
    $('tour-body').textContent = cards[at].body.replace(/\s+/g, ' ').trim();
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
