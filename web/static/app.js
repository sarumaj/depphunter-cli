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
import { $, fmt, escapeHTML } from './dom.js';
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
  linkKind: 'import',     // edges drawn and listed for the selection: import | reference
  theme: 'auto',
};

let model, L, pal, langs, scene, panel, searchItems, defaultHiddenEcosystems, maxDepth = 0;
let config = {};        // /api/config
let slots = [];         // languages holding categorical color slots (colors.assignSlots)
let graphVersion = 0;   // server graph version currently shown
let flashTimer = 0;
let metricsCache = null; // computeMetrics() for the current model, history and since
let focus = null;  // {lit: Set<box>, arcs}
let labels;       // Labels layer over the map
let walker;       // first-person walk mode
let aimX = 0;     // where the walk-mode tooltip was last placed

async function main() {
  const [{ graph, version }, cfg] = await Promise.all([fetchGraph(), fetchConfig()]);
  config = cfg;
  Object.assign(state, { colorBy: cfg.colorBy, heightScale: cfg.heightScale, theme: cfg.theme });
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
  scene.onRender = () => labels.draw();
  walker = new Walker(scene, $('walk-hud'), {
    onAim: (i, x, y) => {
      if (i !== state.hovered) {
        state.hovered = i;
        recolor();
      } else if (x === aimX) return; // the crosshair moves when the panel resizes the map
      aimX = x;
      showTooltip(i, x, y);
    },
    onHit: box => {
      select(box.node);
      updateStatus(`delivered to ${box.node.name}`);
    },
    onSelect: box => select(box.node),
    onToggle: box => {
      const n = box.node;
      toggle(n);
      select(n.kind === 'symbol' && !state.expanded.has(n.parentNode.id) ? n.parentNode : n);
    },
    onExit: () => setWalking(false),
    onRender: () => labels.draw(),
  });
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
  });

  applyTheme();
  bindControls();
  relayout();
  scene.fit(L.bounds);
  updateStatus();
  if (cfg.watch) connectEvents();
  loadLazy('history', setHistory);
  if (cfg.lsp || STATIC?.references) {
    state.referencesStatus = 'loading';
    loadLazy('references', applyReferences);
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

// saveViewSettings stores the view settings - colors, heights, theme, depth and
// filters - in the project config through the server.
async function saveViewSettings() {
  const hiddenIslands = [...state.filters.hiddenEcosystems].map(id => id.replace(/^e:/, ''));
  const stdIslands = model.ecosystems.filter(e => e.std);
  try {
    await saveSettings({
      theme: state.theme,
      colorBy: state.colorBy,
      heightScale: state.heightScale,
      expandDepth: state.level,
      showStd: stdIslands.length > 0 && stdIslands.every(e => !state.filters.hiddenEcosystems.has(e.id)),
      hideLanguages: [...state.filters.hiddenLangs],
      // Standard-library islands follow showStd; save only the other hidden islands.
      hideIslands: hiddenIslands.filter(id => !stdIslands.some(e => e.id === 'e:' + id)),
      pathFilter: state.filters.path,
    });
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
  for (const n of model.byId.values()) {
    if (n.kind === 'dir') maxDepth = Math.max(maxDepth, n.depth + 1);
    if (n.kind === 'file') maxLoc = Math.max(maxLoc, n.loc || 0);
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

function connectEvents() {
  const es = new EventSource('api/events');
  es.onopen = () => setLive(true);
  es.onerror = () => setLive(false);
  // After a reconnect the server may be ahead of us.
  es.addEventListener('hello', e => {
    if (JSON.parse(e.data).version !== graphVersion) queueReload([]);
  });
  es.addEventListener('graph', e => queueReload(JSON.parse(e.data).changed || []));
  es.addEventListener('history', () => loadLazy('history', setHistory));
  es.addEventListener('references', () => loadLazy('references', applyReferences));
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
  state.flash = new Set(changed.map(p => 'f:' + p));
  drawFilters();
  drawLegend();
  relayout();
  if (state.selected) panel.show(state.selected); else panel.close();
  const time = new Date().toLocaleTimeString();
  updateStatus(changed.length ? `updated ${time} · ${fmt.format(changed.length)} changed (highlighted)` : `updated ${time}`);
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => { state.flash = null; recolor(); }, 3000);
}

function setLive(on) {
  const el = $('live');
  el.hidden = false;
  el.classList.toggle('off', !on);
  el.textContent = on ? 'live' : 'reconnecting…';
  el.title = on ? 'Watching the file system; the map updates as files change' : 'Lost connection to depphunter';
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
  const files = model.graph.nodes.filter(n => n.kind === 'file').length;
  const hidden = state.vis.hiddenFiles ? ` · ${fmt.format(state.vis.hiddenFiles)} hidden by filters` : '';
  const refs = state.referencesStatus === 'ready'
    ? ` · ${fmt.format(state.references.edges.length)} references${state.references.partial ? ' (partial)' : ''}`
    : state.referencesStatus === 'loading' ? ' · finding references…' : '';
  const hist = state.history ? ` · ${fmt.format(state.history.commits)} commits${state.history.truncated ? '+' : ''}` :
    state.historyStatus === 'loading' && config.history !== false ? ' · reading git history…' : '';
  $('status-text').textContent = `${fmt.format(files)} files · ${fmt.format(model.root.totalLoc)} lines · ${fmt.format(model.graph.edges.length)} imports${hidden}${hist}${refs}` +
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
  $('depth-label').textContent = `depth ${state.level}/${maxDepth}`;
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

function toggle(n) {
  if (n.kind === 'symbol') n = n.parentNode;
  if (n.kind === 'file' && !n.children.length) return;
  if (n.kind !== 'dir' && n.kind !== 'file') return;
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
  if (n) panel.show(n); else panel.close();
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
  if (!on) scene.requestRender();
}

// ---------------------------------------------------------------- drawing

function relayout() {
  L = layout(model, state);
  scene.setBoxes(L.boxes, baseColors());
  walker.setBoxes(L.boxes);
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
      case 'package': return n.unresolved ? pal.pkgUnresolved : pal.pkg;
    }
    return pal.other;
  });
}

let maxLoc = 1;
// color-by-size uses a square-root scale so mid-sized files stay distinguishable.
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
  scene.setOutline(focus ? focus.selBox : null, pal.select);
  recolor();
  labels.set(L.boxes, focus, state.selected);
}

function recolor() {
  const base = baseColors();
  const sel = state.selected;
  const colors = L.boxes.map((b, i) => {
    let c = base[i];
    const structural = b.kind === 'land' || b.kind === 'terrace';
    if (focus && !structural && !focus.lit.has(b) && !isWithin(b.node, sel)) c = pal.dim;
    if (state.legendLang !== undefined && (b.kind === 'building' || b.kind === 'symbol')) {
      const lang = b.kind === 'symbol' ? b.node.parentNode.lang : b.node.lang;
      const isOther = langs.of(lang) === pal.other;
      if (state.legendLang === null ? !isOther : lang !== state.legendLang) c = pal.dim;
    }
    if (state.flash && (state.flash.has(b.node.id) || (b.kind === 'symbol' && state.flash.has(b.node.parentNode.id)))) c = mix(c, pal.select, 0.5);
    if (i === state.hovered) c = mix(c, pal.select, 0.25);
    return c;
  });
  scene.setColors(colors);
}

const mix = (a, b, t) => '#' + new Color(a).lerp(new Color(b), t).getHexString();

// ---------------------------------------------------------------- legend & tooltip

function drawLegend() {
  const el = $('legend');
  const parts = [];
  const mode = colorMode();
  if (isHistoryMode(mode)) {
    parts.push(historyLegend(mode));
  } else if (mode === 'language') {
    parts.push('<h3>Language</h3><ul>' + langs.legend().map(e =>
      `<li data-lang="${e.lang === null ? '' : escapeHTML(e.lang)}" data-other="${e.lang === null}" class="${isOff(e) ? 'off' : ''}"
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
    li.addEventListener('mouseenter', () => { state.legendLang = li.dataset.other === 'true' ? null : li.dataset.lang; recolor(); });
    li.addEventListener('mouseleave', () => { state.legendLang = undefined; recolor(); });
    li.addEventListener('click', () => {
      const list = li.dataset.other === 'true' ? otherLangs() : [li.dataset.lang];
      setLangsHidden(list, !li.classList.contains('off'));
      state.legendLang = undefined;
      drawFilters();
      applyFilters();
    });
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

function showTooltip(i, x, y) {
  const tip = $('tooltip');
  if (i < 0) { tip.hidden = true; return; }
  const b = L.boxes[i], n = b.node;
  const row = (k, v) => `<div class="t-row">${k} <b>${escapeHTML(String(v))}</b></div>`;
  let html = `<div class="t-title">${escapeHTML(n.path && n.path !== '.' ? n.path : n.name)}</div>`;
  if (n.kind === 'file') html += row('Language', n.lang || 'unknown') + row('Lines', fmt.format(n.loc || 0)) + (n.children.length ? row('Symbols', n.children.length) : '');
  else if (n.kind === 'dir') html += row(b.kind === 'district' ? 'Collapsed directory' : 'Directory', '') + row('Files', fmt.format(n.fileCount)) + row('Lines', fmt.format(n.totalLoc));
  else if (n.kind === 'symbol') html += row(n.symbolKind, `line ${n.line}`);
  else if (n.kind === 'package') html += row('Ecosystem', n.parentNode.name) + (n.version ? row('Version', n.version) : '') + row('Imported by', `${n.importers} files`) + (n.unresolved ? row('⚠', 'not declared in a manifest') : '');
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
  const r = tip.parentElement.getBoundingClientRect();
  const tx = Math.min(x - r.left + 14, r.width - tip.offsetWidth - 8);
  const ty = Math.min(y - r.top + 14, r.height - tip.offsetHeight - 8);
  tip.style.left = tx + 'px';
  tip.style.top = ty + 'px';
}


// ---------------------------------------------------------------- input

function applyTheme() {
  if (state.theme === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
  pal = readPalette();
  langs = languageColors(model, pal, slots);
  scene.setBackground({ water: pal.water, sky: pal.sky, skyTop: pal.skyTop, sea: pal.sea });
  if (L) { scene.setOutline(focus?.selBox, pal.select); refreshFocus(); }
  if (state.selected) panel.show(state.selected); // swatches in the panel
  drawLegend();
  drawFilters();
}

function bindControls() {
  const map = $('map');
  let down = null, hoverFrame = 0, lastMove = null;

  // Walk mode handles the pointer itself (walk.js).
  map.addEventListener('pointerdown', e => { if (walker.active) return; down = { x: e.clientX, y: e.clientY, button: e.button }; map.classList.add('grabbing'); });
  window.addEventListener('pointerup', e => {
    map.classList.remove('grabbing');
    if (!down) return;
    const moved = Math.hypot(e.clientX - down.x, e.clientY - down.y) > 4;
    if (!moved && down.button === 0 && e.target === scene.renderer.domElement) {
      const i = scene.pick(e.clientX, e.clientY);
      select(i >= 0 ? L.boxes[i].node : null);
    }
    down = null;
  });
  map.addEventListener('dblclick', e => {
    if (walker.active) return;
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
      const i = scene.pick(lastMove.clientX, lastMove.clientY);
      map.classList.toggle('hovering', i >= 0);
      if (i !== state.hovered) { state.hovered = i; recolor(); }
      showTooltip(i, lastMove.clientX, lastMove.clientY);
    });
  });
  map.addEventListener('pointerleave', () => { if (walker.active) return; state.hovered = -1; showTooltip(-1); recolor(); });
  map.addEventListener('contextmenu', e => e.preventDefault());

  $('expand-level').onclick = () => setLevel(state.level + 1);
  $('collapse-level').onclick = () => setLevel(state.level - 1);
  $('fit').onclick = () => scene.fit(L.bounds);
  $('walk').onclick = () => setWalking(!walker.active);
  $('rotate-left').onclick = () => scene.setIso(scene.quarter - 1);
  $('rotate-right').onclick = () => scene.setIso(scene.quarter + 1);
  $('help-btn').onclick = () => $('help').showModal();
  $('panel-close').onclick = () => select(null);

  const bindSelect = (id, key, after) => {
    const el = $(id);
    el.value = state[key];
    el.onchange = () => { state[key] = el.value; after(); };
  };
  bindSelect('color-by', 'colorBy', () => { recolor(); drawLegend(); });
  bindSelect('height-scale', 'heightScale', () => { relayout(); drawLegend(); });
  bindSelect('theme', 'theme', applyTheme);
  bindFilters();
  bindSearch();
  bindExport();
  $('path-filter').value = state.filters.path;
  updateFilterBadge();
  if (STATIC) {
    // A static export has no server: nothing to save or open, and only the image to export.
    $('save-settings').hidden = true;
    for (const a of document.querySelectorAll('#export [data-server]')) a.hidden = true;
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
      case 'Escape': select(null); break;
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
  pop.addEventListener('keydown', e => { if (e.key === 'Escape') { setOpen(false); btn.focus(); } });

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
  const choose = r => { close(); input.blur(); reveal(r.node); };
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
  input.addEventListener('input', () => { results = search(searchItems, input.value, state.vis.visible); active = 0; render(); });
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
  document.addEventListener('pointerdown', e => { if (!e.target.closest('.export')) setOpen(false); });
  pop.addEventListener('click', e => { if (e.target.closest('a, button')) setOpen(false); });
}

main().catch(err => {
  $('status-text').textContent = `Failed to load: ${err.message}`;
  console.error(err);
});
