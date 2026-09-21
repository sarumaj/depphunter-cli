import { buildModel, boundaryEdges, isWithin, setReferences } from './model.js';
import { layout } from './layout.js';
import { MapScene } from './scene.js';
import { readPalette, languageColors, assignSlots, sequential } from './colors.js';
import { Panel } from './panel.js';
import { computeVisibility, searchIndex, search } from './filter.js';
import { STATIC, fetchGraph, fetchConfig, fetchLazy, saveSettings } from './data.js';
import { MODES, isHistoryMode, computeMetrics, historyT, timeRange, ago, formatDate } from './history.js';
import { Labels } from './labels.js';
import { Walker } from './walk.js';
import { loadHands } from './hands.js';
import { loadPlants } from './props.js';
import { Bugs } from './bugs.js';
import { Pins } from './pins.js';
import { Routes } from './routes.js';
import { Backpack } from './backpack.js';
import { indexFindings } from './findings.js';
import { $, h, fmt, escapeHTML } from './dom.js';
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
let labels;       // Labels layer over the map
let walker;       // first-person walk mode
let bugs;         // the findings walking the streets in walk mode
let pins;         // the same findings, as markers over the map
let routes;       // the selection's dependencies, laid down as roads
let pack;         // what has been caught (backpack.js)
let aimX = 0;     // where the walk-mode tooltip was last placed

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
  let labelsAt = 0;
  const drawLabels = () => {
    if (walker?.active) {
      const now = performance.now();
      if (now - labelsAt < 90) return;
      labelsAt = now;
    }
    labels.draw();
  };
  scene.onRender = () => { drawLabels(); pins?.follow(); };
  walker = new Walker(scene, $('walk-hud'), {
    onAim: (i, x, y) => {
      if (i !== state.hovered) {
        state.hovered = i;
        recolor();
      } else if (x === aimX) return; // the crosshair moves when the panel resizes the map
      aimX = x;
      showTooltip(i, x, y);
    },
    onHit: (box, tagged) => {
      const tool = walker.tool;
      select(box.node);
      walker.flash(`${box.node.name} ${tool.noun} - ${tagged} ${tool.noun} so far; its dependency trails are lit. ` +
        `Use the ${tool.label.toLowerCase()} on it again for its details`);
    },
    // Using the tool a second time on a tagged building, or Enter: read about what
    // the reticle is on. The panel needs the pointer, so it is freed; a click on the
    // map (or closing the panel) captures it again.
    onInspect: box => {
      const n = box ? box.node : state.selected;
      if (!n) {
        walker.flash(`${walker.tool.verb} a building, then use the tool on it again (or press Enter) for its details`);
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
    tool: () => state.tool,
    onTool: id => { state.tool = id; },
    onRender: drawLabels,
  });
  pins = new Pins(scene);
  routes = new Routes(scene);
  pack = new Backpack(model.root.name, drawPack);
  bugs = new Bugs(scene, {
    // A caught bug reads itself out: the building it belongs to is selected and its
    // finding opened, exactly as a second shot into a building would. It also goes
    // into the backpack, which is where it can be found again afterwards.
    onCatch: (f, node) => {
      pack.add(f, node);
      select(node);
      panel.show(node, false, f.id);
      document.exitPointerLock?.();
      walker.setFrozen(true);
      walker.drawHud();
      const { caught, total } = bugs.counts;
      walker.flash(`${f.severity}: ${f.title} - ${caught} of ${total} caught, and in the backpack. Click the map to keep walking`);
    },
  });
  walker.setBugs(bugs);
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
    onCatch: (f, kept) => {
      if (kept) pack.remove(f.id);
      else pack.add(f, state.findings?.place(f));
    },
  });

  applyStyle(false);
  applyTheme();
  loadHands(); // the walker's hands, fetched while the map is still being looked at
  loadPlants().then(got => got && scene.redress()); // and what grows on the map
  bindControls();
  drawPack();
  relayout();
  scene.fit(L.bounds);
  updateStatus();
  if (cfg.watch) connectEvents();
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
  drawPack();
  placeBugs();
  updateStatus();
  if (state.selected) panel.show(state.selected, true);
}

// placeBugs re-spawns the bugs for the current layout. Boxes change with every depth
// change and every live update, and a bug stands beside its building.
function placeBugs() {
  // Whatever is in the backpack was caught already and stays caught across a relayout,
  // a reload, or a depth change.
  bugs.keepCaught(pack.ids);
  bugs.place(state.findings, L.boxes);
  pins.place(state.findings, L.boxes);
  pins.show(!walker.active && !!state.findings);
  if (walker.active) walker.drawHud();
}

// ---------------------------------------------------------------- the backpack

// drawPack redraws the list and the toolbar button. The button only exists when there
// is something to put in it: a map read without --findings has no bugs to catch.
function drawPack() {
  // The backpack is what says which findings are caught, so whatever changed it has
  // just changed the streets too.
  bugs?.keepCaught(pack.ids);
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

function setPackOpen(on) {
  $('pack').hidden = !on;
  $('pack-btn').setAttribute('aria-expanded', on);
  if (on) drawPack();
}

// ---------------------------------------------------------------- git history

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
  return isHistoryMode(state.colorBy) && !state.history ? 'language' : state.colorBy;
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
const MAX_RECONNECTS = 4;
let events = null, reconnects = 0;

function connectEvents() {
  events?.close();
  const es = events = new EventSource('api/events');
  es.onopen = () => { reconnects = 0; setLive('live'); };
  es.onerror = () => {
    if (++reconnects <= MAX_RECONNECTS) return setLive('reconnecting');
    es.close();
    setLive('stopped');
  };
  // After a reconnect the server may be ahead of us.
  es.addEventListener('hello', e => {
    if (JSON.parse(e.data).version !== graphVersion) queueReload([]);
  });
  es.addEventListener('graph', e => queueReload(JSON.parse(e.data).changed || []));
  es.addEventListener('history', () => loadLazy('history', setHistory));
  es.addEventListener('references', () => loadLazy('references', applyReferences));
  es.addEventListener('findings', () => loadLazy('findings', applyFindings));
}

function queueReload(changed) {
  reloadChain = reloadChain.then(() => reload(changed)).catch(err => {
    console.error(err);
    updateStatus(`update failed: ${err.message}`);
  });
}

async function reload(changed) {
  const { graph, version } = await fetchGraph();
  if (version === graphVersion) return;
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
function setLive(state) {
  const el = $('live');
  el.hidden = false;
  el.classList.toggle('off', state !== 'live');
  el.classList.toggle('stopped', state === 'stopped');
  el.textContent = { live: 'live', reconnecting: 'reconnecting…', stopped: 'depphunter stopped' }[state];
  el.title = state === 'live'
    ? 'Watching the file system; the map updates as files change'
    : state === 'reconnecting' ? 'Lost connection to depphunter' : 'depphunter is no longer running - click to try again';
  el.onclick = state === 'stopped' ? () => { reconnects = 0; setLive('reconnecting'); connectEvents(); } : null;
}

// ---------------------------------------------------------------- screenshot

// saveScreenshot downloads the map as shown, labels included, at the screen's
// resolution.
function saveScreenshot() {
  const map = scene.renderNow();
  const dpr = map.width / map.clientWidth;
  const out = document.createElement('canvas');
  out.width = map.width;
  out.height = map.height;
  const ctx = out.getContext('2d');
  ctx.drawImage(map, 0, 0);
  ctx.scale(dpr, dpr);
  const origin = map.getBoundingClientRect();
  for (const el of labels.visible()) {
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
  out.toBlob(blob => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${model.root.name}.png`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  }, 'image/png');
}

// ---------------------------------------------------------------- editor

async function openFile(path, line = 1) {
  if (!config.editor) {
    // No server-side editor: hand the file to VS Code through its URL handler.
    const abs = config.root.replace(/\\/g, '/') + '/' + path;
    location.href = `vscode://file${abs.startsWith('/') ? '' : '/'}${encodeURI(abs)}:${line}`;
    return;
  }
  const res = await fetch('api/open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Depphunter-Request': '1' },
    body: JSON.stringify({ path, line }),
  });
  if (!res.ok) updateStatus(`could not open editor: ${(await res.text()).trim()}`);
}

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

function setLevel(level, redraw = true) {
  state.level = Math.max(1, Math.min(maxDepth, level));
  state.expanded = new Set([...model.byId.values()].filter(n => n.kind === 'dir' && n.depth < state.level).map(n => n.id));
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
  if (n.kind === 'symbol') n = n.parentNode;
  if (n.kind === 'file' && !n.children.length) return '';
  if (n.kind !== 'dir' && n.kind !== 'file') return '';
  return state.expanded.has(n.id) ? 'close' : 'open';
}

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

function select(n) {
  state.selected = n;
  // The panel would cover the reticle and cannot be reached with the pointer locked,
  // so a walker's selection only opens it once they are back on the map.
  if (n && !walker.active) panel.show(n);
  else if (!n) panel.close();
  refreshFocus();
}

// Expand everything needed to make `n` visible, select it and bring it into view.
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
function setWalking(on) {
  if (on && !walker.active) {
    showTooltip(-1);
    walker.enter(state.selected && rep(state.selected), rep(model.root), L.bounds);
  } else if (!on && walker.active) {
    walker.exit(); // calls back here once it has left
    return;
  }
  $('walk').setAttribute('aria-pressed', on);
  $('map').parentElement.classList.toggle('walking', on);
  // The pins are how findings show on the map; in the street they are bugs instead.
  pins.show(!on && !!state.findings);
  if (on) setPackOpen(false);
  // The map view must never keep the pointer captured: the cursor would be invisible.
  if (!on && document.pointerLockElement) document.exitPointerLock();
  if (on) panel.close();
  else if (state.selected) panel.show(state.selected);
  if (!on) scene.requestRender();
}

// ---------------------------------------------------------------- drawing

// A relayout moves everything; in walk mode the walker is kept next to the block they
// stand on, so a depth change or a live update does not teleport them.
function relayout() {
  const anchor = walker.active ? walker.anchorFor() : null;
  // Box indexes change: whatever was hovered or described is gone.
  state.hovered = -1;
  aimX = NaN;
  showTooltip(-1);
  L = layout(model, state);
  scene.setBoxes(L.boxes, baseColors());
  walker.setBoxes(L.boxes);
  routes?.setLayout(L.boxes); // where a road may run changed with the blocks
  if (bugs) placeBugs();
  const to = anchor && rep(anchor.node);
  if (to) walker.reanchor(anchor, to);
  refreshFocus();
}

/** The box that stands for `n` in the current layout: itself or its nearest visible ancestor. */
function rep(n) {
  for (let p = n; p; p = p.parentNode) {
    const b = L.byNode.get(p.id);
    if (b) return b;
  }
  return null;
}

function baseColors() {
  const mode = colorMode();
  const hm = isHistoryMode(mode) && metrics();
  return L.boxes.map(b => {
    const n = b.node;
    if (hm && (b.kind === 'district' || b.kind === 'building' || b.kind === 'symbol')) {
      const t = historyT(mode, b.kind === 'symbol' ? n.parentNode : n, hm);
      return t === null ? pal.noData : sequential(pal, t);
    }
    switch (b.kind) {
      case 'land': return pal.land;
      case 'terrace': return n.kind === 'file' ? pal.terraceB : (n.depth % 2 ? pal.terraceB : pal.terraceA);
      case 'district':
        return state.colorBy === 'size' ? sequential(pal, sizeT(n.totalLoc / Math.max(1, n.fileCount))) : pal.district;
      case 'building':
        return state.colorBy === 'size' ? sequential(pal, sizeT(n.loc)) : langs.of(n.lang);
      case 'symbol': {
        const f = n.parentNode;
        return state.colorBy === 'size' ? sequential(pal, sizeT(f.loc)) : langs.of(f.lang);
      }
      // A package nothing pins is worth seeing from across the map.
      case 'package': return n.unresolved ? pal.pkgUnresolved : n.floating ? pal.pkgFloating : pal.pkg;
    }
    return pal.other;
  });
}

let maxLoc = 1;
// Color-by-size uses a square-root scale so mid-sized files stay distinguishable.
function sizeT(loc) {
  return Math.sqrt(Math.min(1, (loc || 0) / maxLoc));
}

function refreshFocus() {
  const sel = state.selected;
  const selBox = sel && rep(sel);
  focus = null;
  if (sel && selBox) {
    const { out, in: inc } = boundaryEdges(model, sel, state.linkKind);
    const agg = new Map();
    const add = (from, to, color) => {
      if (!from || !to || from === to) return;
      const key = `${from.i}>${to.i}>${color}`;
      const a = agg.get(key) || { from, to, color, count: 0 };
      a.count++;
      agg.set(key, a);
    };
    const shown = id => {
      const n = model.byId.get(id);
      return state.vis.visible(n) ? rep(n) : null;
    };
    for (const e of out) add(selBox, shown(e.to), pal.edgeOut);
    for (const e of inc) add(shown(e.from), selBox, pal.edgeIn);
    const arcs = [...agg.values()].sort((a, b) => b.count - a.count).slice(0, MAX_ARCS);
    const lit = new Set([selBox]);
    for (const a of arcs) { lit.add(a.from); lit.add(a.to); }
    focus = { lit, arcs, selBox };
  }
  scene.setArcs(focus ? focus.arcs : []);
  // The same edges on the ground, routed between the blocks. The arcs say which
  // buildings are joined; the roads say how you would get there, which is the part
  // a map of a city is supposed to answer.
  routes.set(focus ? focus.arcs : [], focus ? focus.selBox : null);
  scene.setOutline(focus ? focus.selBox : null, pal.select);
  recolor();
  labels.set(L.boxes, focus, state.selected);
}

function recolor() {
  const base = baseColors();
  const sel = state.selected;
  const faded = [];
  const colors = L.boxes.map((b, i) => {
    let c = base[i];
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
function linkKindControl() {
  if (state.referencesStatus === 'loading') return '<p class="hint">finding symbol references…</p>';
  if (state.referencesStatus !== 'ready') return '';
  const button = (kind, label) =>
    `<button data-kind="${kind}" aria-pressed="${state.linkKind === kind}">${label}</button>`;
  return `<div class="link-kind" role="group" aria-label="Edges">${button('import', 'Imports')}${button('reference', 'References')}</div>`;
}

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

// What a marker stands for, without having to click it: how many findings are under
// it, the worst of them, and how much of that is already in the backpack.
//
// Counting what is in the backpack means walking the subtree, and the pointer asks
// once a frame, so the answer is kept until the pin or the backpack changes.
let pinTip = { key: '', html: '' };

function showPinTooltip(pin, x, y) {
  const tip = $('tooltip');
  const n = pin.box.node;
  const key = `${n.id}|${pin.count}|${pack.counts.total}`;
  if (key !== pinTip.key) {
    const row = (k, v) => `<div class="t-row">${k} <b>${escapeHTML(String(v))}</b></div>`;
    const ids = pack.ids;
    const kept = (state.findings?.under(n.id) || []).filter(f => ids.has(f.id)).length;
    pinTip = { key, html: `<div class="t-title">${escapeHTML(n.path && n.path !== '.' ? n.path : n.name)}</div>`
      + row('Findings', fmt.format(pin.count)) + row('Worst', pin.worst || 'unknown')
      + (kept ? row('In the backpack', fmt.format(kept)) : '')
      + '<div class="t-row">Click to read them</div>' };
  }
  tip.innerHTML = pinTip.html;
  tip.hidden = false;
  placeTooltip(tip, x, y);
}

function showTooltip(i, x, y) {
  const tip = $('tooltip');
  if (i < 0) { tip.hidden = true; return; }
  const b = L.boxes[i], n = b.node;
  const row = (k, v) => `<div class="t-row">${k} <b>${escapeHTML(String(v))}</b></div>`;
  let html = `<div class="t-title">${escapeHTML(n.path && n.path !== '.' ? n.path : n.name)}</div>`;
  if (n.kind === 'file') html += row('Language', n.lang || 'unknown') + row('Lines', fmt.format(n.loc || 0)) + (n.children.length ? row('Symbols', n.children.length) : '');
  else if (n.kind === 'dir') html += row(b.kind === 'district' ? 'Collapsed directory' : 'Directory', '') + row('Files', fmt.format(n.fileCount)) + row('Lines', fmt.format(n.totalLoc));
  else if (n.kind === 'symbol') html += row(n.symbolKind, `line ${n.line}`);
  else if (n.kind === 'package') html += row('Ecosystem', n.parentNode.name) + (n.version ? row('Version', n.version) : '') + (n.requested ? row('Requested', n.requested) : '') + row('Imported by', `${n.importers} files`) + (n.unresolved ? row('⚠', 'not declared in a manifest') : '') + (n.floating ? row('⚠', 'not pinned to one version') : '') + (n.transitive ? row('Pulled in by', 'another dependency') : '') + (n.index ? row(n.indexUnknown ? '⚠ Index' : 'Index', n.index.replace(/^https?:\/\//, '')) : '');
  else if (n.kind === 'ecosystem') html += row('Packages', n.children.length);
  const hm = (n.kind === 'file' || n.kind === 'dir') && metrics();
  if (hm) {
    const m = hm.byId.get(n.id);
    html += m
      ? row(`Commits since ${formatDate(state.since)}`, fmt.format(m.commits)) + row('Lines changed', fmt.format(m.churn)) +
        row('Last change', ago(m.last)) + row('Authors', m.authors.size)
      : row('Git history', 'not committed');
  }
  tip.innerHTML = html;
  tip.hidden = false;
  placeTooltip(tip, x, y);
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
 */
function applyStyle(redraw = true) {
  if (state.style === 'city') delete document.documentElement.dataset.style;
  else document.documentElement.dataset.style = state.style;
  scene.setStyle(state.style);
  // The galaxy's void drifts, so the map view has to keep drawing itself; the other
  // two are still, and a still map is drawn once and left alone.
  scene.setAnimated(state.style === 'galaxy');
  if (redraw) {
    applyTheme(); // re-reads the palette, and with it the ground, water and sky
    recolor();
  }
}

function applyTheme() {
  if (state.theme === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
  pal = readPalette();
  langs = languageColors(model, pal, slots);
  scene.setBackground({ water: pal.water, sky: pal.sky, skyTop: pal.skyTop, sea: pal.sea, ground: pal.terraceA, land: pal.land });
  if (L) { scene.setOutline(focus?.selBox, pal.select); refreshFocus(); }
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
    showTooltip(-1); // it would hang over the map for the whole pan
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
  map.addEventListener('dblclick', e => {
    if (walker.active || pins.at(e.clientX, e.clientY)) return; // the pin was the target
    const i = scene.pick(e.clientX, e.clientY);
    if (i >= 0) {
      const n = L.boxes[i].node;
      toggle(n);
      select(n.kind === 'symbol' && !state.expanded.has(n.parentNode.id) ? n.parentNode : n);
    }
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
      if (pin) showPinTooltip(pin, lastMove.clientX, lastMove.clientY);
      else showTooltip(i, lastMove.clientX, lastMove.clientY);
    });
  });
  map.addEventListener('pointerleave', () => {
    if (walker.active) return;
    cancelAnimationFrame(hoverFrame); // else it re-shows the tooltip after the pointer left
    hoverFrame = 0;
    state.hovered = -1;
    showTooltip(-1);
    recolor();
  });
  map.addEventListener('contextmenu', e => e.preventDefault());

  $('expand-level').onclick = () => setLevel(state.level + 1);
  $('collapse-level').onclick = () => setLevel(state.level - 1);
  $('fit').onclick = () => scene.fit(L.bounds);
  $('walk').onclick = () => setWalking(!walker.active);
  $('pack-btn').onclick = () => setPackOpen($('pack').hidden);
  $('pack-close').onclick = () => setPackOpen(false);
  $('pack-clear-fixed').onclick = () => pack.clear(true);
  $('pack-empty').onclick = () => pack.clear();
  $('rotate-left').onclick = () => scene.setIso(scene.quarter - 1);
  $('rotate-right').onclick = () => scene.setIso(scene.quarter + 1);
  $('help-btn').onclick = () => $('help').showModal();
  // Help frees the pointer; closing it captures it again for a walker (a click on
  // Close allows that; Esc does not, then the HUD asks for a click on the map).
  $('help').addEventListener('close', () => walker.active && walker.lockPointer());
  $('panel-close').onclick = () => {
    select(null);
    $('map').focus();
    if (walker.active) walker.lockPointer(); // straight back to the reticle
  };

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

  window.addEventListener('keydown', e => {
    if (e.target.closest('input, select, textarea, dialog') || e.ctrlKey || e.metaKey || e.altKey || walker.owns(e)) return;
    const sel = state.selected;
    switch (e.key) {
      case 'Escape':
        if (document.pointerLockElement) document.exitPointerLock(); // a stray capture
        if (!$('pack').hidden) { setPackOpen(false); break; }
        select(null);
        break;
      case 'b': case 'B':
        if (!$('pack-btn').hidden) setPackOpen($('pack').hidden);
        break;
      case 'f': case 'F': scene.fit(L.bounds); break;
      case 'q': case 'Q': scene.setIso(scene.quarter - 1); break;
      case 'e': case 'E': scene.setIso(scene.quarter + 1); break;
      case '+': case '=': setLevel(state.level + 1); break;
      case '-': case '_': setLevel(state.level - 1); break;
      case '?': document.exitPointerLock?.(); $('help').showModal(); break;
      case '/': document.exitPointerLock?.(); $('search').focus(); break;
      case 'v': case 'V': setWalking(true); break;
      case 'p': case 'P': saveScreenshot(); break;
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

function bindFilters() {
  const btn = $('filters-btn'), pop = $('filters');
  const setOpen = open => { pop.hidden = !open; btn.setAttribute('aria-expanded', open); };
  btn.onclick = () => setOpen(pop.hidden);
  document.addEventListener('pointerdown', e => { if (!e.target.closest('.filters')) setOpen(false); });
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

function bindSearch() {
  const input = $('search'), list = $('search-results');
  let results = [], active = 0;
  const close = () => { list.hidden = true; input.setAttribute('aria-expanded', 'false'); };
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

function bindExport() {
  const btn = $('export-btn'), pop = $('export');
  const setOpen = open => { pop.hidden = !open; btn.setAttribute('aria-expanded', open); };
  btn.onclick = () => setOpen(pop.hidden);
  // The HTML export carries the current view, not whatever the config file holds.
  pop.querySelector('a[href*="format=html"]')?.addEventListener('pointerdown', e => {
    const url = new URL(e.target.closest('a').href, location.href);
    url.searchParams.set('ui', JSON.stringify(viewSettings()));
    e.target.closest('a').href = url.pathname + url.search;
  });
  document.addEventListener('pointerdown', e => { if (!e.target.closest('.export')) setOpen(false); });
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
