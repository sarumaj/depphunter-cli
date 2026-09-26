import { buildModel, focusArcs, expandToLevel, toggles as togglesIn, isWithin, setReferences, unread, fileSize } from './model.js';
import { Fires, MOST } from './fires.js';
import { Flames } from './flames.js';
import { layout, representative } from './layout.js';
import { MapScene } from './scene.js';
import { readPalette, languageColors, assignSlots, boxColor } from './colors.js';
import { Panel } from './panel.js';
import { computeVisibility, searchIndex, search } from './filter.js';
import { STATIC, CLIENT, auth, authed, fetchGraph, fetchConfig, fetchLazy, saveSettings, fetchSession, pushSelection, pushBackpack } from './data.js';
import { MODES, isHistoryMode, effectiveMode, computeMetrics, historyT, timeRange, ago, formatDate } from './history.js';
import { Labels } from './labels.js';
import { Walker } from './walk.js';
import { loadHands } from './hands.js';
import { loadPlants, loadBugs } from './models.js';
import { Bugs, TAKE_MS } from './bugs.js';
import { Pins } from './pins.js';
import { Avatar } from './avatar.js';
import { startTour, startWalkTour, walkTourPending } from './tour.js';
import { Backpack } from './backpack.js';
import { Stash } from './stash.js';
import { indexFindings } from './findings.js';
import { $, h, fmt, escapeHTML, whenUnlocked } from './dom.js';
import { Color } from './vendor/three.module.min.js';

const MAX_ARCS = 400;
const AUTO_ITEMS = 600;

const state = {
  expanded: new Set(),
  level: 2,
  selected: null,
  hovered: -1,
  legendLang: undefined, // language isolated by hovering the legend; null = "Other"
  colorBy: 'language',
  heightScale: 'sqrt',
  filters: { hiddenLangs: new Set(), hiddenEcosystems: new Set(), path: '' },
  vis: null, // computeVisibility() result for the current filters
  history: null,          // git history (internal/history.History), when loaded
  historyStatus: 'loading', // loading | ready | none
  since: 0,               // history modes count changes at or after this unix time
  references: null,       // symbol references from language servers ({edges, servers, partial})
  referencesStatus: 'off', // off | loading | ready | none
  findings: null,         // what the scanners said, indexed onto the model (findings.js)
  findingsStatus: 'off',  // off | loading | ready | none
  linkKind: 'import',     // edges drawn and listed for the selection: import | reference
  theme: 'auto',
  style: 'city',          // what the map is dressed as: city | circuit | galaxy
  tool: '',               // what walk mode holds; empty leaves tools.js its default
};

let model, L, pal, langs, scene, panel, searchItems, defaultHiddenEcosystems, maxDepth = 0, fileCount = 0;
let config = {};        // /api/config
let slots = [];         // languages holding categorical color slots (colors.assignSlots)
let graphVersion = 0;   // server graph version currently shown
let flashTimer = 0;
let metricsCache = null; // computeMetrics() for the current model, history and since
let focus = null;  // {lit: Set<box>, arcs}
let plainly = false; // the map is being drawn without anything the interface put on it
let labels;       // Labels layer over the map
let walker;       // first-person walk mode
let bugs;         // the findings walking the streets in walk mode
let readCaught = 0; // the timer that holds the details back until a catch has played
let fires;        // what is alight, and where it is going (fires.js)
let flames;       // ... and what that looks like, on the map and in the street
let pins;         // the same findings, as markers over the map
let avatar;       // where the walker stands, seen from the map
let pack;         // what has been caught (backpack.js)
let stash;        // what the camera has photographed (stash.js)
let aimX = 0;     // where the walk-mode tooltip was last placed
// Opens the export menu, which bindExport owns; X reaches it from either view.
let openExport = () => {};
// What wants the mouse while it is open. A pointer captured at the reticle goes to the
// canvas and nowhere else, so none of these can be clicked while the walker holds it.
const WANTS_POINTER = ['pack', 'stash', 'export', 'panel'];

async function main() {
  const [{ graph, version }, cfg] = await Promise.all([fetchGraph(), fetchConfig()]);
  config = cfg;
  Object.assign(state, {
    colorBy: cfg.colorBy, heightScale: cfg.heightScale, theme: cfg.theme,
    style: cfg.style || 'city', tool: cfg.tool || '',
  });
  // Saved filters (config ui.hide_languages / hide_islands / path_filter).
  for (const l of cfg.hideLanguages || []) state.filters.hiddenLangs.add(l);
  for (const e of cfg.hideIslands || []) state.filters.hiddenEcosystems.add('e:' + e);
  state.filters.path = cfg.pathFilter || '';
  defaultHiddenEcosystems = new Set();
  setModel(graph, version);
  document.title = `${model.root.name} · depphunter`;
  $('repo-name').textContent = model.root.name;
  setLevel(cfg.expandDepth < 0 ? maxDepth : cfg.expandDepth || autoLevel(), false);

  scene = new MapScene($('map'));
  labels = new Labels($('labels'), scene);
  // Walk mode renders continuously; labels are laid out greedily over all boxes, so
  // they are redrawn a few times a second rather than on every frame. The walker
  // draws its own frames, so its hook must go through the same throttle as the map's
  // renderer - drawing from it directly meant a layout per frame.
  // Implements: REQ-PERF-003
  let labelsAt = 0;
  const drawLabels = () => {
    if (walker?.active) {
      const now = performance.now();
      if (now - labelsAt < 90) return;
      labelsAt = now;
    }
    // Nothing is labelled across what the walker is holding, which is drawn in the
    // canvas under the labels' own layer.
    labels.draw(walker?.active ? scene.handMask() : []);
  };
  scene.onRender = () => { drawLabels(); pins?.follow(); avatar?.follow(); };
  walker = new Walker(scene, $('walk-hud'), {
    onAim: (i, x, y) => {
      if (i !== state.hovered) {
        state.hovered = i;
        recolor();
      } else if (x === aimX) return; // the crosshair moves when the panel resizes the map
      aimX = x;
      tooltip(i >= 0 ? `box:${i}` : null, x, y, showTooltip.bind(null, i));
    },
    // Implements: REQ-HUNT-001, REQ-HUNT-008, REQ-HUNT-009, REQ-TOOL-005
    onHit: (box, tagged) => {
      const tool = walker.primary;
      select(box.node);
      walker.flash(`${box.node.name} ${tool.noun} - ${tagged} ${tool.noun} so far; its dependency trails are lit. ` +
        `Use the ${tool.label.toLowerCase()} on it again for its details`);
    },
    // Using the tool a second time on a tagged building, or Enter: read about what
    // the reticle is on. The panel needs the pointer, so it is freed; a click on the
    // map (or closing the panel) captures it again.
    // Implements: REQ-WALK-021, REQ-HUNT-005, REQ-HUNT-006, REQ-HUNT-007
    onInspect: box => {
      const n = box ? box.node : state.selected;
      if (!n) {
        walker.flash(`${walker.primary.verb} a building, then use the tool on it again (or press Enter) for its details`);
        return;
      }
      select(n);
      panel.show(n);
      document.exitPointerLock?.();
      // The panel has the pointer now; hold the view still until it is given back.
      walker.setFrozen(true);
      walker.flash(`Details of ${n.name} - click the map to keep walking`);
    },
    onExit: () => setWalking(false),
    // Walking on again puts away whatever was being read, so Escape in the street is
    // one key rather than one per thing that might be open.
    onResume: () => { setPackOpen(false); setStashOpen(false); panel.close(); },
    // Everything that wants the mouse, in one place. The walker asks before taking the
    // pointer back, so a panel or a menu is never left with a reticle underneath it
    // swallowing the clicks meant for it.
    // Implements: REQ-WALK-037
    busy: () => !!document.querySelector('dialog[open]')
      || WANTS_POINTER.some(id => !$(id).hidden),
    // Implements: REQ-TOOL-003
    tool: () => state.tool,
    onTool: id => { state.tool = id; },
    // Every use of the camera keeps the frame. It goes into the stash rather than
    // straight to a file: a photograph is a thing to collect and look at, and the ones
    // worth keeping are saved from there one at a time.
    //
    // A photograph is of the city and of nothing else: the hands are left out of it,
    // because the camera cannot be in its own picture, and so is everything the
    // interface has drawn on the map - a module tagged a minute ago should not be lit
    // in a picture of the street it stands on, and the arcs over the rooftops belong
    // to the map view rather than to the place.
    // Implements: REQ-HUNT-034, REQ-HUNT-039, REQ-HUNT-040
    onPhoto: where => {
      plain(() => frame(blob => {
        const it = stash.add(blob, where);
        walker.flash(`Photograph ${it.n} kept - G opens them`);
      }, false));
      return true;
    },
    // What the walker's health is built on: a full backpack is a walker who can stand
    // in a swarm, and an empty one is somebody who should watch their step.
    caught: () => pack?.counts.total ?? 0,
    // The worst thing the scanners said about a module, which is what its beacon and
    // its ring on the tracker are colored by. Nothing when findings are turned off.
    // Implements: REQ-HUNT-004, REQ-HUNT-023
    severityOf: node => state.findings?.rollup(node.id).worst || null,
    onRender: drawLabels,
  });
  pins = new Pins(scene);
  avatar = new Avatar(scene);
  pack = new Backpack(model.root.name, drawPack);
  stash = new Stash(drawStash);
  // The page is the one with a store that outlives the server, so what it remembers
  // is what the session starts from.
  // Implements: REQ-HUNT-028
  pushBackpack(pack.items);
  bugs = new Bugs(scene, {
    // A caught bug reads itself out: the building it belongs to is selected and its
    // finding opened, exactly as a second shot into a building would. It also goes
    // into the backpack, which is where it can be found again afterwards.
    //
    // The reading waits for the catch to finish, though. Every tool takes a bug away
    // in its own manner - reeled down the line, scooped into the net, carried off in a
    // bubble - and that is nearly a second of the one thing in walk mode that happens
    // because the walker did something well. Opening the details on the frame the bug
    // is caught put a panel over it and a blur behind that, so nobody ever saw it:
    // the reward for a good shot was the thing that hid it.
    //
    // So the backpack, the health and the word all land at once, and the panel comes
    // when the bug has finished arriving.
    // Implements: REQ-HUNT-015, REQ-HUNT-016
    onCatch: (f, node) => {
      pack.add(f, node);
      walker.health.caught(pack.counts.total);
      walker.drawHud();
      const { caught, total } = bugs.counts;
      walker.flash(`${f.severity}: ${f.title} - ${caught} of ${total} caught, and in the backpack`);
      clearTimeout(readCaught);
      readCaught = setTimeout(() => {
        // A walk can end, or be left, between the catch and the reading.
        if (!walker.active || walker.dying !== null) return;
        select(node);
        panel.show(node, false, f.id);
        document.exitPointerLock?.();
        walker.setFrozen(true);
        walker.flash(`${f.severity}: ${f.title} - click the map to keep walking`);
      }, TAKE_MS + 120);
    },
  });
  walker.setBugs(bugs);
  // Fire is the map's, not the walker's: a reachable vulnerability burns whether
  // anybody is standing in the street or looking down at the city, and it goes on
  // spreading either way. The walker only carries the thing that puts it out.
  fires = new Fires(model);
  flames = new Flames(scene, MOST);
  flames.show(true);
  walker.setFires(fires);
  panel = new Panel($('panel'), $('panel-body'), {
    model,
    colorOf: lang => langs.of(lang),
    onSelect: n => reveal(n),
    onOpen: openFile,
    linkKind: () => state.linkKind,
    historyOf: node => {
      const hm = metrics();
      return hm && { metric: hm.byId.get(node.id), since: state.since, authors: state.history.authors };
    },
    openLabel: cfg.static ? null : cfg.editor ? 'Open in editor' : 'Open in VS Code',
    // Closing the details gives the pointer back, so walk mode may move again.
    onClose: () => walker.setFrozen(false),
    // `under` is a getter: a directory near the root carries every finding in the
    // repository, and the panel only asks for that list when a reader presses for it.
    findingsOf: node => state.findings && {
      own: state.findings.own(node.id),
      rollup: state.findings.rollup(node.id),
      get under() { return state.findings.under(node.id); },
    },
    caught: id => pack.has(id),
    // Taking a finding from the panel is the same act as netting its bug in the
    // street: it goes in the same backpack, and its bug stops walking. It is the
    // whole of collecting in the map view, where there are no streets to walk.
    // Implements: REQ-HUNT-029, REQ-HUNT-031
    onCatch: (f, kept) => {
      if (kept) pack.remove(f.id);
      else pack.add(f, state.findings?.place(f));
    },
  });

  applyStyle(false);
  applyTheme();
  loadHands(); // the walker's hands, fetched while the map is still being looked at
  loadPlants().then(got => got && scene.redress()); // and what grows on the map
  loadBugs().then(got => got && bugs && placeBugs()); // ... and what walks it
  bindControls();
  drawPack();
  drawStash();
  relayout();
  scene.fit(L.bounds);
  updateStatus();
  // Last, so that what is being explained is already behind the dialog rather than
  // arriving under it.
  startTour();
  // The stream is worth having whether or not the file system is being watched: it
  // is also how a selection made in the editor's side panel, or a backpack changed
  // there, reaches this page. Without a server there is nothing to connect to.
  if (!STATIC) connectEvents();
  loadLazy('history', setHistory);
  if (cfg.lsp || STATIC?.references) {
    state.referencesStatus = 'loading';
    loadLazy('references', applyReferences);
  }
  if (cfg.findings || STATIC?.findings) {
    state.findingsStatus = 'loading';
    loadLazy('findings', applyFindings);
  }
}

// loadLazy fetches a dataset the server computes after startup, polling while it is
// still being computed, and hands the result (or null) to apply.
// Implements: REQ-HIST-007
async function loadLazy(name, apply) {
  for (;;) {
    let v = null;
    try {
      v = await fetchLazy(name);
    } catch (err) {
      console.error(err);
    }
    if (v !== 'pending') return apply(v);
    await new Promise(r => setTimeout(r, 1000));
  }
}

// Implements: REQ-LSP-007
function applyReferences(r) {
  state.references = r;
  state.referencesStatus = r ? 'ready' : 'none';
  setReferences(model, r?.edges);
  if (!r) state.linkKind = 'import';
  refreshFocus();
  drawLegend();
  updateStatus();
  if (state.selected) panel.show(state.selected);
}

// applyFindings places what the scanners reported on the model, sends its bugs out to
// walk the streets, and redraws whatever was already on screen.
function applyFindings(set) {
  state.findings = set ? indexFindings(set, model) : null;
  state.findingsStatus = set ? 'ready' : 'none';
  // Every live update is a chance for something in the backpack to have been fixed.
  pack.reconcile(state.findings);
  // New reports are a new fire. What has been put out for good stays out: light()
  // keeps that list, the way the backpack keeps what has been caught.
  fires.light(state.findings);
  drawPack();
  placeBugs();
  updateStatus();
  if (state.selected) panel.show(state.selected, true);
}

// placeBugs re-spawns the bugs for the current layout. Boxes change with every depth
// change and every live update, and a bug stands beside its building.
/**
 * The fire clock.
 *
 * It runs only while something is alight, which is most of the time never: a map read
 * without --findings, or one whose advisories are all against code nothing calls, has
 * no fire in it and nothing here ever starts. While it does run it is the one place
 * fire advances, so that the same fire is the same age whether it is being watched
 * from the street or from above.
 *
 * Walk mode draws every frame on its own, so out there this only has to do the
 * thinking; on the map it asks for the redraw as well, through the same reason-held
 * animation a drifting style uses.
 */
let fireFrame = 0, fireAt = 0;
function runFire(now) {
  fireFrame = 0;
  const dt = Math.min(0.1, (now - (fireAt || now)) / 1000); // a tab left in the background does not burn down
  fireAt = now;
  fires.step(dt);
  flames.place([...fires.entries()], id => L?.byNode.get(id));
  flames.step(dt);
  if (!walker.active) scene.requestRender();
  if (fires.burning) fireFrame = requestAnimationFrame(runFire);
  else { fireAt = 0; scene.setAnimated(false, 'fire'); }
}

/** Starts the clock if anything is alight and it is not already running. */
function keepBurning() {
  if (!fires?.burning || fireFrame) return;
  scene.setAnimated(true, 'fire');
  fireAt = 0;
  fireFrame = requestAnimationFrame(runFire);
}

function placeBugs() {
  // Whatever is in the backpack was caught already and stays caught across a relayout,
  // a reload, or a depth change.
  bugs.keepCaught(pack.ids);
  bugs.place(state.findings, L.boxes);
  // A relayout moves every building, so the fires move with them; what is alight and
  // how hot is not the layout's business and survives it untouched.
  flames.place([...fires.entries()], id => L.byNode.get(id));
  keepBurning();
  pins.place(state.findings, L.boxes);
  pins.show(!walker.active && !!state.findings);
  if (walker.active) walker.drawHud();
}

// ---------------------------------------------------------------- the backpack

// drawPack redraws the list and the toolbar button. The button only exists when there
// is something to put in it: a map read without --findings has no bugs to catch.
// Implements: REQ-HUNT-029
function drawPack(_pack, fromServer = false) {
  // The backpack is what says which findings are caught, so whatever changed it has
  // just changed the streets too.
  bugs?.keepCaught(pack.ids);
  // ... and whatever else is looking at this server - the editor's side panel - is
  // reading the same catch, so it goes up. Not when it just came down from there.
  if (!fromServer) pushBackpack(pack.items);
  scene?.requestRender();
  const { total, open, fixed } = pack.counts;
  const btn = $('pack-btn'), count = $('pack-count');
  btn.hidden = !(total || state.findings);
  count.hidden = !total;
  count.textContent = total;
  $('pack-summary').textContent = total
    ? `${open} still open${fixed ? `, ${fixed} fixed since` : ''}`
    : '';
  $('pack-empty-note').hidden = total > 0;
  $('pack-clear-fixed').hidden = !fixed;
  $('pack-empty').hidden = !total;
  // Implements: REQ-EXP-015
  $('pack-export').hidden = STATIC || !total;
  const list = $('pack-list');
  list.replaceChildren(...pack.items.map(it => packRow(it)));
}

function packRow(it) {
  const drop = h('button', {
    class: 'drop', type: 'button', title: 'Take it out of the backpack',
    onclick: e => { e.stopPropagation(); pack.remove(it.id); },
  }, '×');
  return h('li', {
    class: `${it.fixed ? 'fixed ' : ''}sev-${it.severity}`,
    title: it.fixed ? 'Gone from the latest scan' : it.title,
    tabindex: '0', role: 'button',
    onclick: () => openCaught(it),
    onkeydown: e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openCaught(it); } },
  },
    h('span', { class: 'sev-dot' }),
    h('span', { class: 'body' },
      h('span', { class: 't' }, it.title),
      h('span', { class: 'w' }, it.where + (it.line ? `:${it.line}` : ''))),
    h('span', { class: 'state' }, it.fixed ? 'fixed' : it.severity),
    drop);
}

// Opening one goes back to where it was caught: the building is revealed and selected
// and the finding opened in the panel, if the scanners still report it.
function openCaught(it) {
  const node = model.byId.get(it.nodeId);
  if (!node) {
    updateStatus(it.fixed ? 'fixed, and no longer on the map' : 'no longer on the map');
    return;
  }
  setWalking(false);
  reveal(node);
  panel.show(node, false, it.fixed ? undefined : it.id);
}

let packing = () => {}; // calls off a backpack still waiting for the pointer (whenUnlocked)

// Implements: REQ-WALK-034, REQ-UI-014
function setPackOpen(on) {
  packing();
  const show = () => {
    $('pack').hidden = !on;
    $('pack-btn').setAttribute('aria-expanded', on);
    if (on) drawPack();
  };
  // Reading is reading, on the street as much as over the map: the walker holds still
  // and lets go of the pointer while the backpack is open, and picks both up again
  // when it closes. It opens once the pointer is free, not while the lock is still
  // letting go.
  if (!walker?.active) return show();
  if (on) {
    walker.setFrozen(true);
    packing = whenUnlocked(show);
  } else {
    show();
    walker.lockPointer();
  }
}

// ---------------------------------------------------------------- git history

// Implements: REQ-HIST-010
function setHistory(h) {
  state.history = h;
  state.historyStatus = h ? 'ready' : 'none';
  const range = h && timeRange(h);
  if (range && !(state.since >= range.from && state.since <= range.to)) state.since = range.from;
  metricsCache = null;
  const group = $('history-modes');
  group.hidden = !h;
  group.label = h ? 'Git history' : 'Git history (loading…)';
  for (const o of group.querySelectorAll('option')) o.disabled = !h;
  if (!h && isHistoryMode(state.colorBy)) $('color-by').value = state.colorBy = 'language';
  recolor();
  drawLegend();
  updateStatus();
  if (state.selected) panel.show(state.selected);
}

/** Metrics for the current model and range, or null without history. */
function metrics() {
  if (!state.history) return null;
  if (!metricsCache || metricsCache.model !== model || metricsCache.since !== state.since) {
    metricsCache = { ...computeMetrics(model, state.history, state.since), model, since: state.since };
  }
  return metricsCache;
}

/** The color mode in effect: history modes fall back to language until loaded. */
function colorMode() {
  return effectiveMode(state.colorBy, state.history);
}

// The view as the settings format describes it: colors, heights, theme, depth and
// filters. Saved to the project config, and sent with an HTML export so the exported
// page opens looking like the map on screen.
function viewSettings() {
  const hiddenIslands = [...state.filters.hiddenEcosystems].map(id => id.replace(/^e:/, ''));
  const stdIslands = model.ecosystems.filter(e => e.std);
  return {
    theme: state.theme,
    style: state.style,
    colorBy: state.colorBy,
    heightScale: state.heightScale,
    expandDepth: state.level,
    showStd: stdIslands.length > 0 && stdIslands.every(e => !state.filters.hiddenEcosystems.has(e.id)),
    hideLanguages: [...state.filters.hiddenLangs],
    // Standard-library islands follow showStd; save only the other hidden islands.
    hideIslands: hiddenIslands.filter(id => !stdIslands.some(e => e.id === 'e:' + id)),
    pathFilter: state.filters.path,
    tool: state.tool,
  };
}

// Implements: REQ-CFG-012
async function saveViewSettings() {
  try {
    await saveSettings(viewSettings());
    updateStatus(`settings saved to ${config.configFile}`);
  } catch (err) {
    updateStatus(`could not save: ${err.message}`);
  }
}

// setModel installs a graph, carrying over what the user chose on the previous one:
// expanded directories, selection, filters and language colors.
// Implements: REQ-WATCH-005
function setModel(graph, version) {
  const prev = model;
  model = buildModel(graph);
  graphVersion = version;
  searchItems = searchIndex(model);
  slots = assignSlots(model, slots);
  setReferences(model, state.references?.edges);
  if (panel) panel.model = model;
  maxDepth = 0;
  maxLoc = 1;
  fileCount = 0;
  for (const n of model.byId.values()) {
    if (n.kind === 'dir') maxDepth = Math.max(maxDepth, n.depth + 1);
    if (n.kind === 'file') { maxLoc = Math.max(maxLoc, n.loc || 0); fileCount++; }
  }
  for (const e of model.ecosystems) {
    if (e.std && !config.showStd && !prev?.byId.has(e.id)) {
      state.filters.hiddenEcosystems.add(e.id);
      defaultHiddenEcosystems.add(e.id);
    }
  }
  if (prev) {
    for (const n of model.byId.values()) {
      if (n.kind === 'dir' && !prev.byId.has(n.id) && n.depth < state.level) state.expanded.add(n.id);
    }
    state.selected = state.selected ? model.byId.get(state.selected.id) || null : null;
  }
  state.vis = computeVisibility(model, state.filters);
}

// ---------------------------------------------------------------- live updates

let reloadChain = Promise.resolve();

// How many failed reconnects before giving up: the browser retries an EventSource by
// itself, and once depphunter has stopped (Ctrl+C, or a crash) that is an endless
// stream of console errors instead of an answer.
// Implements: REQ-UI-002
const MAX_RECONNECTS = 4;
let events = null, reconnects = 0;
// Whether this page has been greeted by a stream before, which is what tells a
// reconnection from the first connection of all.
let greeted = false;

// Implements: REQ-UI-002, REQ-WATCH-004, REQ-HIST-008, REQ-LSP-005, REQ-FND-022, REQ-SRV-012, REQ-SRV-017
function connectEvents() {
  events?.close();
  // An EventSource cannot set a header, so in embed mode the token rides in the
  // query string instead (data.js).
  const es = events = new EventSource(authed('api/events'));
  es.onopen = () => { reconnects = 0; setLive('live'); };
  es.onerror = () => {
    if (++reconnects <= MAX_RECONNECTS) return setLive('reconnecting');
    es.close();
    setLive('stopped');
  };
  // After a reconnect the server may be ahead of us - or may be a different server,
  // started under a page that was left open, numbering its versions from one again.
  // `resumed` is the stream saying nothing has been announced since the last event
  // this page saw; failing that it asks, and the entity tag makes the asking free
  // when the answer is the graph it already has.
  //
  // The first greeting is not a reconnection: main() has just read everything.
  es.addEventListener('hello', e => {
    const first = !greeted;
    greeted = true;
    if (first || JSON.parse(e.data).resumed) return;
    queueReload([]);
    adoptSession();
  });
  es.addEventListener('graph', e => queueReload(JSON.parse(e.data).changed || []));
  es.addEventListener('history', () => loadLazy('history', setHistory));
  es.addEventListener('references', () => loadLazy('references', applyReferences));
  es.addEventListener('findings', () => loadLazy('findings', applyFindings));
  // What somebody else picked, and what somebody else took out of the backpack. Our
  // own changes come back too, named as ours, and are left alone.
  es.addEventListener('selection', e => {
    const { id, origin } = JSON.parse(e.data);
    if (origin === CLIENT) return;
    const n = id ? model.byId.get(id) : null;
    if (n) reveal(n);
    else if (!id) select(null, true);
  });
  es.addEventListener('backpack', e => {
    if (JSON.parse(e.data).origin === CLIENT) return;
    adoptBackpack();
  });
}

// Implements: REQ-HUNT-029
/** Takes the backpack as the server now has it; replace redraws through onChange. */
async function adoptBackpack() {
  const session = await fetchSession();
  if (session) pack.replace(session.backpack);
}

/**
 * Catches up on what was shared while this page was not listening. A stream that
 * came back without resuming may have missed a selection or a change to the catch,
 * and neither is announced again.
 */
async function adoptSession() {
  const session = await fetchSession();
  if (!session) return;
  pack.replace(session.backpack);
  const n = session.selected ? model.byId.get(session.selected) : null;
  if (n && n !== state.selected) reveal(n);
}

function queueReload(changed) {
  reloadChain = reloadChain.then(() => reload(changed)).catch(err => {
    console.error(err);
    updateStatus(`update failed: ${err.message}`);
  });
}

// Implements: REQ-MAP-044, REQ-MAP-046, REQ-WATCH-005, REQ-WATCH-006, REQ-SRV-017
async function reload(changed) {
  const { graph, version } = await fetchGraph();
  // 304: the server has what this page already holds, whatever the version counter
  // says. A server restarted under an open page numbers from one again, and the
  // graph it found is usually the same graph.
  if (!graph || version === graphVersion) {
    graphVersion = version;
    return;
  }
  setModel(graph, version);
  langs = languageColors(model, pal, slots);
  // The findings are indexed onto the model, and this is a new one.
  if (state.findings) {
    const { all, sources, partial } = state.findings;
    state.findings = indexFindings({ findings: all, sources, partial }, model);
  }
  state.flash = new Set(changed.map(p => 'f:' + p));
  drawFilters();
  drawLegend();
  relayout();
  if (state.selected) panel.show(state.selected, true); else panel.close();
  const time = new Date().toLocaleTimeString();
  updateStatus(changed.length ? `updated ${time} · ${fmt.format(changed.length)} changed (highlighted)` : `updated ${time}`);
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => { state.flash = null; recolor(); }, 3000);
}

/** state: 'live' | 'reconnecting' | 'stopped'; stopped can be clicked to try again. */
// Implements: REQ-UI-002, REQ-UI-003
function setLive(state) {
  const el = $('live');
  el.hidden = false;
  el.classList.toggle('off', state !== 'live');
  el.classList.toggle('stopped', state === 'stopped');
  // Connected without --watch: the map does not follow the files, but the selection
  // and the backpack still travel, so saying "live" would promise the wrong thing.
  const connected = config.watch ? 'live' : 'connected';
  el.textContent = { live: connected, reconnecting: 'reconnecting…', stopped: 'depphunter stopped' }[state];
  el.title = state !== 'live'
    ? state === 'reconnecting' ? 'Lost connection to depphunter' : 'depphunter is no longer running - click to try again'
    : config.watch
      ? 'Watching the file system; the map updates as files change'
      : 'Connected to depphunter. Started without --watch, so the map does not follow the files';
  el.onclick = state === 'stopped' ? () => { reconnects = 0; setLive('reconnecting'); connectEvents(); } : null;
}

// ---------------------------------------------------------------- photographs

/**
 * The photographs, as a contact sheet. They live for the session only, so the note
 * under them says so and every one carries the button that writes it to a file.
 *
 * From the street each one can also be put up on the camera and looked at there,
 * which is the difference between a list of thumbnails and something the walker is
 * carrying about with them. There is nothing to put it on from the map, so that
 * button is only drawn while walking - setWalking redraws this when that changes.
 */
function drawStash() {
  const btn = $('stash-btn'), list = $('stash-list');
  btn.hidden = !stash.count;
  $('stash-count').hidden = !stash.count;
  $('stash-count').textContent = stash.count;
  $('stash-summary').textContent = stash.count
    ? `${stash.count} this session - save the ones you want`
    : '';
  $('stash-empty-note').hidden = !!stash.count;
  // Only worth saying where there is both something to show and somewhere to show it.
  $('stash-show-note').hidden = !stash.count || !walker?.active;
  list.replaceChildren(...stash.items.map(it => h('li', {},
    h('img', { src: it.url, alt: '', loading: 'lazy' }),
    h('div', { class: 'what' },
      h('b', {}, `Photograph ${it.n}`),
      h('span', {}, it.where || new Date(it.at).toLocaleTimeString())),
    walker?.active && h('button', {
      class: 'link',
      title: 'Put it up on the camera and look at it there',
      onclick: () => { setStashOpen(false); walker.showPhoto(it); },
    }, 'show'),
    h('button', { class: 'link keep', onclick: () => stash.save(it.id, model.root.name) }, 'save'),
    h('button', { class: 'close', title: 'Let go of this one', onclick: () => stash.remove(it.id) }, '×'))));
  if (!stash.count) setStashOpen(false);
}

let stashing = () => {}; // as `packing`, for the photographs

// Implements: REQ-UI-014
function setStashOpen(on) {
  stashing();
  const show = () => {
    $('stash').hidden = !on;
    $('stash-btn').setAttribute('aria-expanded', on);
  };
  // Looking at them is reading, the same as the backpack: the walker holds still and
  // lets the pointer go while they are open, and takes both back when they close.
  if (!walker?.active) return show();
  if (on) {
    walker.setFrozen(true);
    stashing = whenUnlocked(show);
  } else {
    show();
    walker.lockPointer();
  }
}

// ---------------------------------------------------------------- screenshot

/** The view as it stands, written out as a PNG the browser downloads. */
// Implements: REQ-EXP-011, REQ-EXP-012, REQ-HUNT-041
function saveScreenshot() {
  frame(blob => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${model.root.name}.png`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  });
}

/**
 * Runs `then` with the map in its own colors: nothing lit by a selection, nothing
 * dimmed by one, no outline and no dependency arcs. Everything is put back afterwards,
 * including if `then` throws, because a map left plain would be a selection that has
 * silently stopped showing.
 *
 * It is drawn twice more than it would otherwise be - once to strip it and once to put
 * it back - which is nothing for something that happens on a click.
 *
 * Implements: REQ-HUNT-040
 */
function plain(then) {
  const was = focus;
  plainly = true;
  focus = null;
  scene.setArcs([]);
  scene.setOutline(null, pal.select);
  recolor();
  try {
    return then();
  } finally {
    plainly = false;
    focus = was;
    scene.setArcs(focus ? focus.arcs : []);
    scene.setOutline(focus ? focus.selBox : null, pal.select);
    recolor();
  }
}

/**
 * The view as it stands, as a PNG blob. `then` is called with it once the canvas has
 * encoded it, which it does off the main thread.
 *
 * In walk mode the labels are not drawn over the street, so only the view itself is
 * copied - which for a photograph is the whole of what was in the frame anyway.
 *
 * `hands` is what separates the two things this is asked for. A screenshot is the
 * screen, hands and all; a photograph is what the camera was pointed at, and a camera
 * held up in the corner of its own picture is a mistake nobody makes twice.
 *
 * Implements: REQ-EXP-011, REQ-HUNT-033, REQ-HUNT-039, REQ-HUNT-041
 */
function frame(then, hands = true) {
  const map = scene.renderNow(hands);
  const dpr = map.width / map.clientWidth;
  const out = document.createElement('canvas');
  out.width = map.width;
  out.height = map.height;
  const ctx = out.getContext('2d');
  ctx.drawImage(map, 0, 0);
  ctx.scale(dpr, dpr);
  const origin = map.getBoundingClientRect();
  for (const el of walker.active ? [] : labels.visible()) {
    const r = el.getBoundingClientRect();
    const css = getComputedStyle(el);
    const x = r.left - origin.left, y = r.top - origin.top;
    ctx.fillStyle = css.backgroundColor;
    ctx.strokeStyle = css.borderTopColor;
    ctx.beginPath();
    ctx.roundRect(x, y, r.width, r.height, 4);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = css.color;
    ctx.font = `${css.fontWeight} ${css.fontSize} ${css.fontFamily}`;
    ctx.textBaseline = 'middle';
    ctx.fillText(el.textContent, x + parseFloat(css.paddingLeft) + 1, y + r.height / 2);
  }
  out.toBlob(then, 'image/png');
}

// ---------------------------------------------------------------- editor

// Implements: REQ-SRV-005, REQ-SRV-008
async function openFile(path, line = 1) {
  if (!config.editor) {
    // No server-side editor: hand the file to VS Code through its URL handler.
    const abs = config.root.replace(/\\/g, '/') + '/' + path;
    location.href = `vscode://file${abs.startsWith('/') ? '' : '/'}${encodeURI(abs)}:${line}`;
    return;
  }
  const res = await fetch('api/open', {
    method: 'POST',
    headers: auth({ 'Content-Type': 'application/json', 'X-Depphunter-Request': '1' }),
    body: JSON.stringify({ path, line }),
  });
  if (!res.ok) updateStatus(`could not open editor: ${(await res.text()).trim()}`);
}

// Implements: REQ-MAP-059
function updateStatus(note = '') {
  const hidden = state.vis.hiddenFiles ? ` · ${fmt.format(state.vis.hiddenFiles)} hidden by filters` : '';
  const refs = state.referencesStatus === 'ready'
    ? ` · ${fmt.format(state.references.edges.length)} references${state.references.partial ? ' (partial)' : ''}`
    : state.referencesStatus === 'loading' ? ' · finding references…' : '';
  const hist = state.history ? ` · ${fmt.format(state.history.commits)} commits${state.history.truncated ? '+' : ''}` :
    state.historyStatus === 'loading' && config.history !== false ? ' · reading git history…' : '';
  const found = state.findings
    ? ` · ${fmt.format(state.findings.all.length)} findings${state.findings.partial ? ' (partial)' : ''}`
    : state.findingsStatus === 'loading' ? ' · reading scanner reports…' : '';
  $('status-text').textContent = `${fmt.format(fileCount)} files · ${fmt.format(model.root.totalLoc)} lines · ${fmt.format(model.graph.edges.length)} imports${hidden}${hist}${refs}${found}` +
    (note ? ` · ${note}` : '');
}

// ---------------------------------------------------------------- filters

function applyFilters() {
  state.vis = computeVisibility(model, state.filters);
  if (state.selected && !state.vis.visible(state.selected)) select(null);
  updateFilterBadge();
  relayout();
  drawLegend();
  updateStatus();
}

// The badge counts filters beyond the defaults, so std-lib islands hidden at start do
// not show up as filters.
function updateFilterBadge() {
  const ecoChanges = model.ecosystems.filter(e => state.filters.hiddenEcosystems.has(e.id) !== defaultHiddenEcosystems.has(e.id)).length;
  const active = state.filters.hiddenLangs.size + ecoChanges + (state.filters.path.trim() ? 1 : 0);
  $('filter-count').hidden = !active;
  $('filter-count').textContent = active;
}

// Languages folded into the legend's "Other" entry.
function otherLangs() {
  const inLegend = new Set(langs.legend().filter(e => e.lang).map(e => e.lang));
  return ['', ...model.languages.map(l => l.lang).filter(l => !inLegend.has(l))];
}

function setLangsHidden(list, hide) {
  for (const l of list) hide ? state.filters.hiddenLangs.add(l) : state.filters.hiddenLangs.delete(l);
}

// Make a node visible again by lifting the filters that hide it.
function unhide(n) {
  if (state.vis.visible(n)) return false;
  for (let p = n; p; p = p.parentNode) {
    if (p.kind === 'file') state.filters.hiddenLangs.delete(p.lang || '');
    if (p.kind === 'ecosystem') state.filters.hiddenEcosystems.delete(p.id);
  }
  state.vis = computeVisibility(model, state.filters);
  if (!state.vis.visible(n)) state.filters.path = $('path-filter').value = '';
  drawFilters();
  applyFilters();
  return true;
}

function drawFilters() {
  const counts = new Map();
  for (const n of model.byId.values()) if (n.kind === 'file') counts.set(n.lang || '', (counts.get(n.lang || '') || 0) + 1);
  const check = (id, label, checked, meta, swatch) => `<li><label><input type="checkbox" data-id="${escapeHTML(id)}" ${checked ? 'checked' : ''}>
    ${swatch ? `<span class="swatch" style="background:${swatch}"></span>` : ''}${escapeHTML(label)}<span class="meta">${meta}</span></label></li>`;
  $('lang-list').innerHTML = [...counts.entries()].sort((a, b) => b[1] - a[1])
    .map(([l, c]) => check(l, l || 'unknown', !state.filters.hiddenLangs.has(l), fmt.format(c), langs.of(l))).join('');
  $('eco-list').innerHTML = model.ecosystems
    .map(e => check(e.id, e.name, !state.filters.hiddenEcosystems.has(e.id), fmt.format(e.children.length), null)).join('');
}

// ---------------------------------------------------------------- state changes

// Implements: REQ-MAP-023
function setLevel(level, redraw = true) {
  ({ level: state.level, expanded: state.expanded } = expandToLevel(model, level));
  $('depth-label').textContent = `depth ${state.level}/${Math.max(1, maxDepth)}`;
  if (redraw) relayout();
}

// Deepest level at which the map shows at most AUTO_ITEMS buildings and districts.
function autoLevel() {
  const perLevel = level => {
    let items = 0;
    for (const n of model.byId.values()) {
      if (n.kind === 'file' && n.parentNode.depth < level) items++;
      else if (n.kind === 'dir' && n.depth === level) items++;
    }
    return items;
  };
  let level = 1;
  while (level < maxDepth && perLevel(level + 1) <= AUTO_ITEMS) level++;
  return level;
}

/** What toggling `n` would do: 'open', 'close', or '' when there is nothing to open. */
function toggles(n) {
  return togglesIn(n, state.expanded);
}

// Implements: REQ-MAP-022
function toggle(n) {
  if (n.kind === 'symbol') n = n.parentNode;
  if (!toggles(n)) {
    updateStatus(`${n.name} has nothing to open`);
    return;
  }
  if (state.expanded.has(n.id)) {
    state.expanded.delete(n.id);
    if (state.selected && state.selected !== n && isWithin(state.selected, n)) state.selected = n;
  } else {
    state.expanded.add(n.id);
  }
  relayout();
}

function select(n, fromServer = false) {
  state.selected = n;
  if (!fromServer) pushSelection(n?.id);
  // The panel would cover the reticle and cannot be reached with the pointer locked,
  // so a walker's selection only opens it once they are back on the map.
  if (n && !walker.active) panel.show(n);
  else if (!n) panel.close();
  refreshFocus();
}

// Expand everything needed to make `n` visible, select it and bring it into view.
// Implements: REQ-WALK-019
function reveal(n) {
  let changed = false;
  for (let p = n.parentNode; p; p = p.parentNode) {
    if ((p.kind === 'dir' || p.kind === 'file') && !state.expanded.has(p.id)) {
      state.expanded.add(p.id);
      changed = true;
    }
  }
  if (unhide(n)) changed = true;
  state.selected = n;
  if (changed) relayout();
  select(n);
  const b = rep(n);
  if (b && walker.active) walker.teleport(b);
  else if (b) scene.centerOn(b.x, b.y + b.h / 2, b.z);
}

// Walk mode: the map seen in first person on a small planet (walk.js).
// The node the walker was last sent to. Entering walk mode resumes where they were
// standing; picking a building on the map first is how you say "take me there
// instead", so a selection that has moved on since wins over the remembered spot.
let sentTo = null;

function walkTarget() {
  const id = state.selected?.id || null;
  const same = id === sentTo;
  sentTo = id;
  return same ? null : state.selected && rep(state.selected);
}

// What the toolbar cannot do from the street: rotating and fitting move the map's own
// camera, which nobody is looking through while walking, so both did nothing silently.
/**
 * The controls that only mean anything on the map, marked `data-map-only` where they
 * are written rather than listed by id here, so adding one is a word in the markup.
 *
 * They are taken away in walk mode rather than greyed out. Greying says "not now",
 * which is a thing worth saying about something a walker might reasonably reach for;
 * none of these are. Rotating, fitting and stepping the depth all move or rebuild the
 * map's own camera and layout, and the walker is standing in that layout - fitting the
 * map to the screen while somebody is in the street is not a disabled action, it is a
 * question nobody asked. A toolbar that shrinks to what is usable is also shorter to
 * read, which matters more in the street than on the map, because reading it there
 * costs the mouse.
 */
const mapOnly = () => document.querySelectorAll('[data-map-only]');

/**
 * Something on screen wants the mouse - a menu, the help, the backpack. In the street
 * that means the same thing every time: the walker holds still where they stand and
 * the pointer comes back, so the thing that asked for it can be worked.
 *
 * It is a function because every one of them was doing it separately and one of them
 * was not. Export froze the walker when it was opened with the X key and did nothing
 * at all when its button was clicked, which is the same menu opening two ways and
 * behaving differently - and the way anybody actually opens it was the broken one.
 *
 * Implements: REQ-WALK-021, REQ-WALK-024, REQ-WALK-034, REQ-UI-014
 */
function readAway() {
  if (!walker?.active) return;
  walker.setFrozen(true);
  document.exitPointerLock?.();
}

// Implements: REQ-WALK-001, REQ-WALK-023, REQ-UI-011, REQ-UI-013
function setWalking(on) {
  if (on && !walker.active) {
    hideTooltip();
    // A first walk is explained on the way in, where there is something to try it on -
    // and before the pointer is taken rather than after, because asking for the
    // reticle and giving it straight back leaves the mouse fighting the dialog. The
    // walker stands still with the pointer free until the last card is out of the way,
    // and takes both back then.
    const teaching = walkTourPending();
    // The side panel and the backpack go before the walker asks for the pointer, not
    // after: it will not take the pointer from under either (hooks.busy), and leaving
    // walk mode reopens the panel on whatever is selected - so walking in again,
    // after dying above all, came back with the mouse free.
    // Implements: REQ-WALK-010, REQ-WALK-037
    setPackOpen(false);
    panel.close();
    walker.enter(walkTarget(), rep(model.root), L.bounds, !teaching);
    // A first walk is also flown in (walker.startArrival): held, the flight waits at
    // its top, so the cards are read over the city and the flight goes on once they
    // are closed and the pointer is taken.
    if (teaching) {
      walker.setFrozen(true);
      startWalkTour(false, () => walker.active && walker.lockPointer());
    }
  } else if (!on && walker.active) {
    walker.exit(); // calls back here once it has left
    return;
  }
  $('walk').setAttribute('aria-pressed', on);
  $('map').parentElement.classList.toggle('walking', on);
  for (const el of mapOnly()) el.hidden = on;
  // The counters would otherwise sit across the tool row in the corner they share.
  // Nothing can be moved out of the way without covering something else, so they
  // stack instead: the readout joins the top of the row's own column.
  const status = $('status');
  const home = on ? $('walk-hud').querySelector('.w-foot') : $('map').parentElement;
  if (status.parentElement !== home) home[on ? 'prepend' : 'append'](status);
  // The pins are how findings show on the map; in the street they are bugs instead.
  pins.show(!on && !!state.findings);
  // ... and the walker, who is only worth drawing when you are not being them.
  avatar.set(walker.stance(), pal.avatar);
  avatar.show(!on);
  // The photographs gain and lose their "show" button with the street.
  if (stash.count) drawStash();
  // The map view must never keep the pointer captured: the cursor would be invisible.
  if (!on && document.pointerLockElement) document.exitPointerLock();
  if (!on && state.selected) panel.show(state.selected);
  if (!on) scene.requestRender();
}

// ---------------------------------------------------------------- drawing

// A relayout moves everything; in walk mode the walker is kept next to the block they
// stand on, so a depth change or a live update does not teleport them.
// Implements: REQ-WALK-025
function relayout() {
  const anchor = walker.active ? walker.anchorFor() : null;
  // Box indexes change: whatever was hovered or described is gone.
  state.hovered = -1;
  aimX = NaN;
  hideTooltip();
  L = layout(model, state);
  scene.setBoxes(L.boxes, baseColors());
  walker.setBoxes(L.boxes);
  if (bugs) placeBugs();
  const to = anchor && rep(anchor.node);
  if (to) walker.reanchor(anchor, to);
  avatar.set(walker.stance(), pal.avatar);
  avatar.show(!walker.active);
  refreshFocus();
}

/** The box that stands for `n` in the current layout: itself or its nearest visible ancestor. */
// Implements: REQ-MAP-009
function rep(n) {
  return representative(L.byNode, n);
}

// Implements: REQ-MAP-037, REQ-MAP-051, REQ-MAP-058, REQ-HIST-010, REQ-HIST-013
function baseColors() {
  const mode = colorMode();
  const hm = isHistoryMode(mode) && metrics();
  return L.boxes.map(b => boxColor(b, { mode, pal, langs, sizeT, hm, historyT }));
}

let maxLoc = 1;
// Color-by-size uses a square-root scale so mid-sized files stay distinguishable.
function sizeT(loc) {
  return Math.sqrt(Math.min(1, (loc || 0) / maxLoc));
}

// Implements: REQ-MAP-012, REQ-MAP-013, REQ-MAP-026
function refreshFocus() {
  focus = focusArcs(model, state.selected, {
    kind: state.linkKind, rep, visible: state.vis.visible,
    outColor: pal.edgeOut, inColor: pal.edgeIn, max: MAX_ARCS,
  });
  scene.setArcs(focus ? focus.arcs : []);
  scene.setOutline(focus ? focus.selBox : null, pal.select);
  recolor();
  labels.set(L.boxes, focus, state.selected);
}

// Implements: REQ-MAP-025
function recolor() {
  const base = baseColors();
  const sel = state.selected;
  const faded = [];
  const colors = L.boxes.map((b, i) => {
    let c = base[i];
    // Plainly: the city in its own colors, with nothing the interface has done to it
    // (plain, above), which is what a photograph is of.
    if (plainly) {
      faded[i] = false;
      return c;
    }
    const structural = b.kind === 'land' || b.kind === 'terrace';
    if (focus && !structural && !focus.lit.has(b) && !isWithin(b.node, sel)) c = pal.dim;
    if (state.legendLang !== undefined && (b.kind === 'building' || b.kind === 'symbol')) {
      const lang = b.kind === 'symbol' ? b.node.parentNode.lang : b.node.lang;
      const isOther = langs.of(lang) === pal.other;
      if (state.legendLang === null ? !isOther : lang !== state.legendLang) c = pal.dim;
    }
    faded[i] = c === pal.dim;
    if (state.flash && (state.flash.has(b.node.id) || (b.kind === 'symbol' && state.flash.has(b.node.parentNode.id)))) c = mix(c, pal.select, 0.5);
    if (i === state.hovered) c = mix(c, pal.select, 0.25);
    return c;
  });
  scene.setColors(colors, faded);
}

const mix = (a, b, t) => '#' + new Color(a).lerp(new Color(b), t).getHexString();

// ---------------------------------------------------------------- legend & tooltip

// Implements: REQ-MAP-014, REQ-MAP-032, REQ-A11Y-005, REQ-A11Y-006
function drawLegend() {
  const el = $('legend');
  state.legendLang = undefined; // the <li> under the pointer is replaced: no mouseleave follows
  const parts = [];
  const mode = colorMode();
  if (isHistoryMode(mode)) {
    parts.push(historyLegend(mode));
  } else if (mode === 'language') {
    parts.push('<h3>Language</h3><ul>' + langs.legend().map(e =>
      `<li data-lang="${e.lang === null ? '' : escapeHTML(e.lang)}" data-other="${e.lang === null}" class="${isOff(e) ? 'off' : ''}"
         tabindex="0" role="button" aria-pressed="${!isOff(e)}"
         title="Click to ${isOff(e) ? 'show' : 'hide'}"><span class="swatch" style="background:${e.color}"></span>${escapeHTML(e.label)}</li>`).join('') + '</ul>');
  } else {
    parts.push(`<h3>File size</h3><div class="ramp" style="background:linear-gradient(90deg,${pal.seq.join(',')})"></div>
      <div class="ramp-labels"><span>0</span><span>${fmt.format(maxLoc)} lines</span></div>`);
  }
  // Islands keep their own three colors in every mode, so they get a key of their own:
  // the tooltip and the panel say which state a package is in, but only here is it
  // said what the paint means. Shown once there is an island to paint.
  if (model.ecosystems.some(e => e.children.length)) {
    parts.push(`<h3>Packages</h3><div class="pkg-key">
      <span title="Declared in a manifest"><i class="swatch" style="background:${pal.pkg}"></i>resolved</span>
      <span title="Not fixed to one version: it moves when installed again"><i class="swatch" style="background:${pal.pkgFloating}"></i>floating</span>
      <span title="Not found in any manifest"><i class="swatch" style="background:${pal.pkgUnresolved}"></i>unresolved</span></div>`);
  }
  parts.push('<h3>Selection</h3>' + linkKindControl() + `<div class="edge-key">
      <span><i class="line" style="background:${pal.edgeOut}"></i>depends on</span>
      <span><i class="line" style="background:${pal.edgeIn}"></i>used by</span></div>`);
  const scaleName = { sqrt: '√ lines of code', linear: 'lines of code', log: 'log lines of code' }[state.heightScale];
  parts.push(`<p>Height: ${scaleName}. Grey blocks are collapsed directories; islands are external dependencies.</p>`);
  el.innerHTML = parts.join('');
  bindSinceSlider();
  for (const b of el.querySelectorAll('.link-kind button')) {
    b.addEventListener('click', () => {
      state.linkKind = b.dataset.kind;
      refreshFocus();
      drawLegend();
      if (state.selected) panel.show(state.selected);
    });
  }

  for (const li of el.querySelectorAll('li[data-lang]')) {
    // Hovering or focusing an entry isolates that language; clicking or pressing
    // Enter/Space hides and shows it.
    const isolate = on => { state.legendLang = on ? (li.dataset.other === 'true' ? null : li.dataset.lang) : undefined; recolor(); };
    li.addEventListener('mouseenter', () => isolate(true));
    li.addEventListener('mouseleave', () => isolate(false));
    li.addEventListener('focus', () => isolate(true));
    li.addEventListener('blur', () => isolate(false));
    const toggleLang = () => {
      const list = li.dataset.other === 'true' ? otherLangs() : [li.dataset.lang];
      setLangsHidden(list, !li.classList.contains('off'));
      state.legendLang = undefined;
      drawFilters();
      applyFilters();
    };
    li.addEventListener('click', toggleLang);
    li.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleLang(); } });
  }
}

function isOff(entry) {
  return entry.lang === null ? otherLangs().every(l => state.filters.hiddenLangs.has(l)) : state.filters.hiddenLangs.has(entry.lang);
}

// The Imports / References switch, once language servers have answered.
// Implements: REQ-LSP-007
function linkKindControl() {
  if (state.referencesStatus === 'loading') return '<p class="hint">finding symbol references…</p>';
  if (state.referencesStatus !== 'ready') return '';
  const button = (kind, label) =>
    `<button data-kind="${kind}" aria-pressed="${state.linkKind === kind}">${label}</button>`;
  return `<div class="link-kind" role="group" aria-label="Edges">${button('import', 'Imports')}${button('reference', 'References')}</div>`;
}

// Implements: REQ-HIST-011, REQ-HIST-013
function historyLegend(mode) {
  const hm = metrics();
  const { from, to } = hm.range;
  const ramp = `<div class="ramp" style="background:linear-gradient(90deg,${pal.seq.join(',')})"></div>`;
  const noData = text => `<div class="no-data"><span class="swatch" style="background:${pal.noData}"></span>${text}</div>`;
  if (mode === 'age') {
    return `<h3>Last change</h3>${ramp}
      <div class="ramp-labels"><span>${escapeHTML(ago(from))}</span><span>${escapeHTML(ago(to))}</span></div>
      ${noData('not committed')}`;
  }
  const unit = { commits: 'commits', churn: 'lines', authors: 'authors' }[mode];
  const total = hm.byId.get(model.root.id)?.commits || 0;
  return `<h3>${MODES[mode].title} per file</h3>${ramp}
    <div class="ramp-labels"><span>0</span><span>${fmt.format(hm.max[mode])} ${unit}</span></div>
    ${noData('no commits in range')}
    <label class="since">Since <input type="range" id="since" min="${from}" max="${to}" step="86400" value="${state.since}"
      aria-describedby="since-label"></label>
    <div class="hint" id="since-label">${escapeHTML(formatDate(state.since))} · ${fmt.format(total)} commits to current files${state.history.truncated ? ' (history truncated)' : ''}</div>`;
}

// The since slider recolors live while dragging and redraws the legend on release.
// Implements: REQ-HIST-011
function bindSinceSlider() {
  const slider = $('since');
  if (!slider) return;
  let frame = 0;
  slider.addEventListener('input', () => {
    state.since = +slider.value;
    $('since-label').textContent = formatDate(state.since);
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(recolor);
  });
  slider.addEventListener('change', () => {
    drawLegend();
    if (state.selected) panel.show(state.selected);
    $('since').focus();
  });
}

// The tooltip waits a moment before it appears.
//
// A panel of numbers thrown under the pointer the instant it crosses a roof is in
// the way of the map it is describing, and crossing the city is most of what the
// pointer does. Waiting until it settles on one building means the panel turns up
// when it was wanted and stays out of the way when it was not - and once it is up,
// moving on to the next building swaps it straight away, because by then the reader
// has said they are reading.
const TIP_DELAY = 240;

// key names what is being described (null: nothing), paint draws it at a position.
const tipState = { key: null, timer: 0, open: false, x: 0, y: 0, paint: null };

function tooltip(key, x = 0, y = 0, paint = null) {
  Object.assign(tipState, { x, y, paint });
  if (key === null) {
    clearTimeout(tipState.timer);
    Object.assign(tipState, { key: null, timer: 0, open: false, paint: null });
    $('tooltip').hidden = true;
    return;
  }
  const same = key === tipState.key;
  tipState.key = key;
  if (tipState.open) {
    tipState.paint(tipState.x, tipState.y);
  } else if (!same) {
    clearTimeout(tipState.timer);
    tipState.timer = setTimeout(() => {
      tipState.open = true;
      tipState.paint?.(tipState.x, tipState.y);
    }, TIP_DELAY);
  }
}

/** Takes the tooltip away, and the wait with it. */
const hideTooltip = () => tooltip(null);

// The card is written once and then only moved.
//
// The pointer asks for it once a frame while it is up, and rebuilding a string for
// the HTML parser each time says the same thing over and over. What it says is keyed
// on the element instead, so whichever of the two kinds of card wrote it last can
// tell whether the other has been in since.
function paint(tip, key, html) {
  if (tip.dataset.card !== key) {
    tip.innerHTML = html();
    tip.dataset.card = key;
  }
  tip.hidden = false;
}

// What a marker stands for, without having to click it: how many findings are under
// it, the worst of them, and how much of that is already in the backpack.
//
// Counting what is in the backpack means walking the subtree, which is the other
// reason not to do it once a frame.
function showPinTooltip(pin, x, y) {
  const tip = $('tooltip');
  const n = pin.box.node;
  paint(tip, `pin:${n.id}|${pin.count}|${pack.counts.total}`, () => {
    const row = (k, v) => `<div class="t-row">${k} <b>${escapeHTML(String(v))}</b></div>`;
    const ids = pack.ids;
    const kept = (state.findings?.under(n.id) || []).filter(f => ids.has(f.id)).length;
    return `<div class="t-title">${escapeHTML(n.path && n.path !== '.' ? n.path : n.name)}</div>`
      + row('Findings', fmt.format(pin.count)) + row('Worst', pin.worst || 'unknown')
      + (kept ? row('In the backpack', fmt.format(kept)) : '')
      + '<div class="t-row">Click to read them</div>';
  });
  placeTooltip(tip, x, y);
}

function showTooltip(i, x, y) {
  const tip = $('tooltip');
  const b = L.boxes[i], n = b?.node;
  if (!n) { tip.hidden = true; return; } // the layout moved under the wait
  paint(tip, `box:${n.id}|${b.kind}|${graphVersion}|${state.since}`, () => card(b, n));
  placeTooltip(tip, x, y);
}

// Implements: REQ-MAP-024, REQ-MAP-060, REQ-A11Y-002, REQ-HIST-014
function card(b, n) {
  const row = (k, v) => `<div class="t-row">${k} <b>${escapeHTML(String(v))}</b></div>`;
  let html = `<div class="t-title">${escapeHTML(n.path && n.path !== '.' ? n.path : n.name)}</div>`;
  // A file nothing read has no lines to report, so it reports its size instead:
  // "Lines 0" would have been a claim about the file rather than about the reading.
  if (n.kind === 'file') html += row('Language', n.lang || 'unknown') + (unread(n) ? row('Size', fileSize(n.bytes)) : row('Lines', fmt.format(n.loc || 0))) + (n.children.length ? row('Symbols', n.children.length) : '');
  else if (n.kind === 'dir') html += row(b.kind === 'district' ? 'Collapsed directory' : 'Directory', '') + row('Files', fmt.format(n.fileCount)) + row('Lines', fmt.format(n.totalLoc));
  else if (n.kind === 'symbol') html += row(n.symbolKind, `line ${n.line}`);
  else if (n.kind === 'package') html += row('Ecosystem', n.parentNode.name) + (n.version ? row('Version', n.version) : '') + (n.requested ? row('Requested', n.requested) : '') + row('Imported by', `${n.importers} files`) + (n.private ? row('Private', 'yours; nothing about it is asked of anyone') : '') + (n.unresolved ? row('⚠', 'not declared in a manifest') : '') + (n.floating ? row('⚠', 'not pinned to one version') : '') + (n.transitive ? row('Pulled in by', 'another dependency') : '') + (n.index ? row(n.indexUnknown ? '⚠ Index' : 'Index', n.index.replace(/^https?:\/\//, '')) : '');
  else if (n.kind === 'ecosystem') html += row('Packages', n.children.length);
  const hm = (n.kind === 'file' || n.kind === 'dir') && metrics();
  if (hm) {
    const m = hm.byId.get(n.id);
    html += m
      ? row(`Commits since ${formatDate(state.since)}`, fmt.format(m.commits)) + row('Lines changed', fmt.format(m.churn)) +
        row('Last change', ago(m.last)) + row('Authors', m.authors.size)
      : row('Git history', 'not committed');
  }
  return html;
}

// Beside the pointer, or below the reticle while walking; flipped to the other side
// when it would not fit, so it never covers what it describes.
function placeTooltip(tip, x, y) {
  const r = tip.parentElement.getBoundingClientRect();
  const gap = walker.active ? 46 : 14;
  const fit = (v, size, max) => (v + gap + size <= max ? v + gap : Math.max(8, Math.min(v - gap - size, max - size - 8)));
  tip.style.left = fit(x - r.left, tip.offsetWidth, r.width) + 'px';
  tip.style.top = fit(y - r.top, tip.offsetHeight, r.height) + 'px';
}


// ---------------------------------------------------------------- input

/**
 * What the map is dressed as. The style picks the environment's colors out of the
 * stylesheet (the ground, the water, the sky) and tells the scene which painter to
 * use; the colors that carry data are the language and history palettes, which do
 * not change with it. redraw is false only while the map is first being built.
 * Implements: REQ-MAP-049, REQ-MAP-052
 */
function applyStyle(redraw = true) {
  if (state.style === 'city') delete document.documentElement.dataset.style;
  else document.documentElement.dataset.style = state.style;
  scene.setStyle(state.style);
  if (redraw) {
    applyTheme(); // re-reads the palette, and with it the ground, water and sky
    recolor();
  }
}

// Implements: REQ-MAP-039
function applyTheme() {
  if (state.theme === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
  pal = readPalette();
  langs = languageColors(model, pal, slots);
  scene.setBackground({ water: pal.water, sky: pal.sky, skyTop: pal.skyTop, sea: pal.sea, ground: pal.terraceA, land: pal.land });
  if (L) { scene.setOutline(focus?.selBox, pal.select); refreshFocus(); }
  avatar?.set(walker.stance(), pal.avatar); // its color is the theme's too
  if (bugs && L) placeBugs(); // the bugs' colors come from the stylesheet too
  if (state.selected) panel.show(state.selected); // swatches in the panel
  drawLegend();
  drawFilters();
}

function bindControls() {
  const map = $('map');
  let down = null, hoverFrame = 0, lastMove = null;

  // Walk mode handles the pointer itself (walk.js).
  map.addEventListener('pointerdown', e => {
    if (walker.active) return;
    down = { x: e.clientX, y: e.clientY, button: e.button };
    map.classList.add('grabbing');
    hideTooltip(); // it would hang over the map for the whole pan
  });
  window.addEventListener('pointerup', e => {
    map.classList.remove('grabbing');
    if (!down) return;
    const moved = Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4;
    if (!moved && down.button === 0 && e.target === scene.renderer.domElement) {
      // A pin stands above the roof it belongs to, so the ray that would hit the
      // building goes past it; it is tried first, and hitting one opens what it is
      // there for rather than the building's stats.
      const pin = pins.at(e.clientX, e.clientY);
      if (pin) {
        select(pin.box.node);
        panel.revealFindings();
      } else {
        const i = scene.pick(e.clientX, e.clientY);
        select(i >= 0 ? L.boxes[i].node : null);
      }
    }
    down = null;
  });
  // Implements: REQ-MAP-022
  map.addEventListener('dblclick', e => {
    if (walker.active || pins.at(e.clientX, e.clientY)) return; // the pin was the target
    const i = scene.pick(e.clientX, e.clientY);
    // On a building, the second click opens it; on open ground there is nothing to
    // open, and walking in is what one is usually after down there anyway. V still
    // does it from anywhere, and M is the way back.
    if (i < 0) { setWalking(true); return; }
    const n = L.boxes[i].node;
    toggle(n);
    select(n.kind === 'symbol' && !state.expanded.has(n.parentNode.id) ? n.parentNode : n);
  });
  map.addEventListener('pointermove', e => {
    lastMove = e;
    if (walker.active || hoverFrame || (down && e.buttons)) return;
    hoverFrame = requestAnimationFrame(() => {
      hoverFrame = 0;
      const pin = pins.at(lastMove.clientX, lastMove.clientY);
      const i = pin ? -1 : scene.pick(lastMove.clientX, lastMove.clientY);
      map.classList.toggle('hovering', !!pin || i >= 0);
      if (i !== state.hovered) { state.hovered = i; recolor(); }
      const { clientX: mx, clientY: my } = lastMove;
      if (pin) tooltip(`pin:${pin.box.node.id}`, mx, my, showPinTooltip.bind(null, pin));
      else tooltip(i >= 0 ? `box:${i}` : null, mx, my, showTooltip.bind(null, i));
    });
  });
  map.addEventListener('pointerleave', () => {
    if (walker.active) return;
    cancelAnimationFrame(hoverFrame); // else it re-shows the tooltip after the pointer left
    hoverFrame = 0;
    state.hovered = -1;
    hideTooltip();
    recolor();
  });
  map.addEventListener('contextmenu', e => e.preventDefault());

  $('expand-level').onclick = () => setLevel(state.level + 1);
  $('collapse-level').onclick = () => setLevel(state.level - 1);
  $('fit').onclick = () => scene.fit(L.bounds);
  $('walk').onclick = () => setWalking(!walker.active);
  $('pack-btn').onclick = () => setPackOpen($('pack').hidden);
  $('pack-close').onclick = () => setPackOpen(false);
  $('stash-btn').onclick = () => setStashOpen($('stash').hidden);
  $('stash-close').onclick = () => setStashOpen(false);
  $('stash-empty').onclick = () => stash.clear();
  $('pack-clear-fixed').onclick = () => pack.clear(true);
  $('pack-empty').onclick = () => pack.clear();
  // Links, as the export menu's are, so in embed mode the token has to be on them.
  for (const a of $('pack-export').querySelectorAll('a')) a.href = authed(a.getAttribute('href'));
  $('rotate-left').onclick = () => scene.setIso(scene.quarter - 1);
  $('rotate-right').onclick = () => scene.setIso(scene.quarter + 1);
  $('reset-view').onclick = () => scene.reset(L.bounds);
  $('help-btn').onclick = () => { readAway(); $('help').showModal(); };
  // The help's own way back to the introduction, for anyone who skipped it or wants it
  // again. One modal at a time, so the help is closed before the other opens.
  // Implements: REQ-UI-010
  $('help-tour').onclick = () => {
    $('help').close();
    // Whichever introduction fits where the reader is: the street has its own.
    if (walker.active) startWalkTour(true, () => walker.active && walker.lockPointer());
    else startTour(true);
  };
  // Help frees the pointer; closing it captures it again for a walker (a click on
  // Close allows that; Esc does not, then the HUD asks for a click on the map).
  // Implements: REQ-WALK-024
  $('help').addEventListener('close', () => walker.active && walker.lockPointer());
  $('panel-close').onclick = () => {
    select(null);
    $('map').focus();
    if (walker.active) walker.lockPointer(); // straight back to the reticle
  };

  // Implements: REQ-CFG-011
  const bindSelect = (id, key, after) => {
    const el = $(id);
    el.value = state[key];
    el.onchange = () => { state[key] = el.value; after(); };
  };
  bindSelect('color-by', 'colorBy', () => { recolor(); drawLegend(); });
  bindSelect('height-scale', 'heightScale', () => { relayout(); drawLegend(); });
  bindSelect('theme', 'theme', applyTheme);
  bindSelect('style', 'style', applyStyle);
  bindFilters();
  bindSearch();
  bindExport();
  $('path-filter').value = state.filters.path;
  updateFilterBadge();
  // Implements: REQ-EXP-008, REQ-EXP-012
  if (STATIC) {
    // A static export has no server: nothing to save or open, and only the image to export.
    $('save-settings').hidden = true;
    for (const a of document.querySelectorAll('[data-server]')) a.hidden = true;
  } else {
    $('save-settings').onclick = saveViewSettings;
    $('save-settings').title = `Save color, height, theme, depth and filters to ${config.configFile}`;
  }
  $('screenshot').onclick = saveScreenshot;
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => state.theme === 'auto' && applyTheme());

  // Implements: REQ-A11Y-003, REQ-MAP-017, REQ-MAP-019
  window.addEventListener('keydown', e => {
    if (e.target.closest('input, select, textarea, dialog') || e.ctrlKey || e.metaKey || e.altKey || walker.owns(e)) return;
    const sel = state.selected;
    switch (e.key) {
      case 'Escape':
        if (document.pointerLockElement) document.exitPointerLock(); // a stray capture
        if (!$('stash').hidden) { setStashOpen(false); break; }
        if (!$('pack').hidden) { setPackOpen(false); break; }
        select(null);
        break;
      case 'g': case 'G':
        // The photographs, from the street as much as from the map.
        if (!$('stash-btn').hidden) setStashOpen($('stash').hidden);
        break;
      case 'b': case 'B':
        // The backpack needs the pointer, and in walk mode the reticle has it: opening
        // it there gives the pointer back and holds the view still, the same as reading
        // a building's details does, and closing it takes both back.
        if (!$('pack-btn').hidden) setPackOpen($('pack').hidden);
        break;
      // Fitting, rotating and stepping the depth all move the map's own camera, which
      // is not the one in use while walking, so they belong to the map view alone.
      case 'Home': if (!walker.active) scene.fit(L.bounds); break;
      case 'r': case 'R': if (!walker.active) scene.reset(L.bounds); break;
      case 'q': case 'Q': if (!walker.active) scene.setIso(scene.quarter - 1); break;
      case 'e': case 'E': if (!walker.active) scene.setIso(scene.quarter + 1); break;
      case '+': case '=': setLevel(state.level + 1); break;
      case '-': case '_': setLevel(state.level - 1); break;
      case '?': readAway(); $('help').showModal(); break;
      case '/': document.exitPointerLock?.(); $('search').focus(); break;
      case 'v': case 'V': setWalking(true); break;
      case 'p': case 'P': saveScreenshot(); break;
      // Saving and exporting are toolbar buttons, and in walk mode the toolbar is
      // behind a captured pointer - so both have a key. The export menu wants the
      // pointer to pick from it and hands it back the way the backpack does; saving
      // needs none, and reports into the status line the walk HUD already carries.
      // Implements: REQ-WALK-034
      case 'x': case 'X': openExport(); break;
      case 'k': case 'K': if (!STATIC) saveViewSettings(); break;
      case 'o': case 'O': {
        if (STATIC) break; // no server, no editor
        const f = sel?.kind === 'symbol' ? sel.parentNode : sel;
        if (f?.kind === 'file') openFile(f.path, sel.line || 1);
        break;
      }
      case 'Enter': if (sel) { toggle(sel); select(sel); } break;
      case 'Backspace': if (sel?.parentNode) reveal(sel.parentNode); break;
      default: return;
    }
    e.preventDefault();
  });
}

// Implements: REQ-MAP-032, REQ-MAP-033, REQ-MAP-034
function bindFilters() {
  const btn = $('filters-btn'), pop = $('filters');
  // `back` is whether closing it is a way back to the street. Shutting it on purpose
  // is; shutting it because the pointer has gone to another control is not, and taking
  // the reticle back there would snatch the mouse out of the control being reached for
  // - which is the whole of what made the toolbar unusable from the street. A click on
  // the map needs no help from here: it asks for the pointer by itself.
  //
  // Opening waits for the pointer lock to let go (whenUnlocked), and the click-outside
  // handler only looks at a menu that is showing, so an event still delivered to the
  // locked canvas cannot shut it on the way in.
  let pending = () => {};
  const setOpen = (open, back = true) => {
    pending();
    const show = () => {
      pop.hidden = !open;
      btn.setAttribute('aria-expanded', open);
    };
    if (open) {
      readAway();
      pending = whenUnlocked(show);
    } else {
      show();
      if (back && walker?.active) walker.lockPointer();
    }
  };
  btn.onclick = () => setOpen(pop.hidden);
  document.addEventListener('pointerdown', e => { if (!pop.hidden && !e.target.closest('.filters')) setOpen(false, false); });
  btn.parentElement.addEventListener('keydown', e => {
    if (e.key === 'Escape' && !pop.hidden) { e.stopPropagation(); setOpen(false); btn.focus(); }
  });

  const onCheck = (listId, set) => $(listId).addEventListener('change', e => {
    const id = e.target.dataset.id;
    if (e.target.checked) set().delete(id); else set().add(id);
    applyFilters();
  });
  onCheck('lang-list', () => state.filters.hiddenLangs);
  onCheck('eco-list', () => state.filters.hiddenEcosystems);
  $('langs-all').onclick = () => { state.filters.hiddenLangs.clear(); drawFilters(); applyFilters(); };
  $('langs-none').onclick = () => { setLangsHidden(['', ...model.languages.map(l => l.lang)], true); drawFilters(); applyFilters(); };

  let timer = 0;
  $('path-filter').addEventListener('input', e => {
    clearTimeout(timer);
    timer = setTimeout(() => { state.filters.path = e.target.value; applyFilters(); }, 250);
  });
  drawFilters();
}

// Implements: REQ-MAP-031
function bindSearch() {
  const input = $('search'), list = $('search-results');
  let results = [], active = 0;
  const close = () => { list.hidden = true; input.setAttribute('aria-expanded', 'false'); };
  // Implements: REQ-WALK-024
  const choose = r => {
    close();
    input.blur();
    reveal(r.node);
    if (walker.active) walker.lockPointer(); // back to the reticle after a search
  };
  const kindLabel = { file: 'file', dir: 'dir', symbol: 'symbol', package: 'package' };
  const render = () => {
    if (!input.value.trim()) return close();
    list.innerHTML = results.length
      ? results.map((r, i) => `<li role="option" id="sr-${i}" aria-selected="${i === active}" data-i="${i}">
          ${r.node.kind === 'file' ? `<span class="swatch" style="background:${langs.of(r.node.lang)}"></span>` : ''}
          <span class="name">${escapeHTML(r.name)}</span><span class="ctx">${escapeHTML(r.context)}</span>
          <span class="badge">${r.node.symbolKind || kindLabel[r.node.kind]}</span></li>`).join('')
      : '<li class="empty">No matches among visible nodes</li>';
    list.hidden = false;
    input.setAttribute('aria-expanded', 'true');
    input.setAttribute('aria-activedescendant', results.length ? `sr-${active}` : '');
  };
  // Debounced: the index can hold 100k entries, and every keystroke would rank them.
  // Implements: REQ-PERF-002
  let typing = 0;
  input.addEventListener('input', () => {
    clearTimeout(typing);
    typing = setTimeout(() => { results = search(searchItems, input.value, state.vis.visible); active = 0; render(); }, 120);
  });
  input.addEventListener('focus', () => input.value.trim() && render());
  input.addEventListener('blur', () => setTimeout(close, 150));
  input.addEventListener('keydown', e => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (results.length) active = (active + (e.key === 'ArrowDown' ? 1 : results.length - 1)) % results.length;
      render();
    } else if (e.key === 'Enter' && results[active]) {
      choose(results[active]);
    } else if (e.key === 'Escape') {
      input.value = '';
      close();
      input.blur();
    } else return;
    e.preventDefault();
  });
  list.addEventListener('pointerdown', e => {
    const li = e.target.closest('li[data-i]');
    if (li) { e.preventDefault(); choose(results[+li.dataset.i]); }
  });
}

// Implements: REQ-UI-014, REQ-EXP-005, REQ-EXP-009
function bindExport() {
  const btn = $('export-btn'), pop = $('export');
  // `back` as in the filters above: shut on purpose it is a way back to the street,
  // shut because the pointer went elsewhere in the page it is not, and taking the
  // reticle back then would snatch the mouse out of whatever was being reached for.
  // Opening waits for the pointer lock to let go, as the filters' does.
  let pending = () => {};
  const setOpen = (open, back = true, then) => {
    pending();
    const show = () => {
      pop.hidden = !open;
      btn.setAttribute('aria-expanded', open);
      then?.();
    };
    if (open) {
      readAway();
      pending = whenUnlocked(show);
    } else {
      show();
      if (back && walker?.active) walker.lockPointer();
    }
  };
  btn.onclick = () => setOpen(pop.hidden);
  /**
   * Opens the menu, from the map or from the street. It always opens rather than
   * toggling, because Esc and a click outside already close it, and a key that does
   * one or the other depending on what is on screen is a key you press twice.
   */
  openExport = () => {
    if (btn.hidden) return;
    setOpen(true, true, () => btn.focus());
  };
  // These are links, not fetches, so in embed mode the token has to be on them.
  for (const a of pop.querySelectorAll('a[href^="api/"]')) a.href = authed(a.getAttribute('href'));
  // The HTML export carries the current view, not whatever the config file holds.
  pop.querySelector('a[href*="format=html"]')?.addEventListener('pointerdown', e => {
    const url = new URL(e.target.closest('a').href, location.href);
    url.searchParams.set('ui', JSON.stringify(viewSettings()));
    e.target.closest('a').href = url.pathname + url.search;
  });
  document.addEventListener('pointerdown', e => {
    if (!pop.hidden && !e.target?.closest?.('.export')) setOpen(false, false);
  });
  // On the group, not the popover: the button keeps the focus while the menu is open.
  btn.parentElement.addEventListener('keydown', e => {
    if (e.key === 'Escape' && !pop.hidden) { e.stopPropagation(); setOpen(false); btn.focus(); }
  });
  pop.addEventListener('click', e => { if (e.target.closest('a, button')) setOpen(false); });
}

main().catch(err => {
  $('status-text').textContent = `Failed to load: ${err.message}`;
  console.error(err);
});
