import { buildModel, boundaryEdges, isWithin } from './model.js';
import { layout } from './layout.js';
import { MapScene } from './scene.js';
import { readPalette, languageColors, sequential } from './colors.js';
import { Panel } from './panel.js';

const $ = id => document.getElementById(id);
const fmt = new Intl.NumberFormat();
const MAX_ARCS = 400;
const MAX_LABELS = 160;
const AUTO_ITEMS = 600;

const state = {
  expanded: new Set(),
  level: 2,
  selected: null,
  hovered: -1,
  legendLang: undefined, // language isolated by hovering the legend; null = "Other"
  colorBy: 'language',
  heightScale: 'sqrt',
  showStd: false,
  theme: 'auto',
};

let model, L, pal, langs, scene, panel, maxDepth = 0;
let focus = null;  // {lit: Set<box>, arcs}
let labels = [];   // label candidates for the current layout/selection

async function main() {
  const [graph, cfg] = await Promise.all([getJSON('api/graph'), getJSON('api/config')]);
  Object.assign(state, {
    colorBy: cfg.colorBy, heightScale: cfg.heightScale, showStd: cfg.showStd, theme: cfg.theme,
  });

  model = buildModel(graph);
  document.title = `${model.root.name} · depphunter`;
  $('repo-name').textContent = model.root.name;
  for (const n of model.byId.values()) {
    if (n.kind === 'dir') maxDepth = Math.max(maxDepth, n.depth + 1);
    if (n.kind === 'file') maxLoc = Math.max(maxLoc, n.loc || 0);
  }
  setLevel(cfg.expandDepth < 0 ? maxDepth : cfg.expandDepth || autoLevel(), false);

  scene = new MapScene($('map'));
  scene.onRender = drawLabels;
  panel = new Panel($('panel'), $('panel-body'), {
    model,
    colorOf: lang => langs.of(lang),
    onSelect: n => reveal(n),
  });

  bindControls();
  applyTheme();
  relayout();
  scene.fit(L.bounds);

  const files = graph.nodes.filter(n => n.kind === 'file').length;
  $('status').textContent = `${fmt.format(files)} files · ${fmt.format(model.root.totalLoc)} lines · ${fmt.format(graph.edges.length)} imports`;
}

async function getJSON(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: ${res.status} ${await res.text()}`);
  return res.json();
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
  if (n.kind === 'package' && n.parentNode.std && !state.showStd) {
    state.showStd = true;
    $('show-std').checked = true;
    changed = true;
  }
  state.selected = n;
  if (changed) relayout();
  select(n);
  const b = rep(n);
  if (b) scene.centerOn(b.x, b.y + b.h / 2, b.z);
}

// ---------------------------------------------------------------- drawing

function relayout() {
  L = layout(model, state);
  scene.setBoxes(L.boxes, baseColors());
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
  return L.boxes.map(b => {
    const n = b.node;
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
// Colour-by-size uses a square-root scale so mid-sized files stay distinguishable.
function sizeT(loc) {
  return Math.sqrt(Math.min(1, (loc || 0) / maxLoc));
}

function refreshFocus() {
  const sel = state.selected;
  const selBox = sel && rep(sel);
  focus = null;
  if (sel && selBox) {
    const { out, in: inc } = boundaryEdges(model, sel);
    const agg = new Map();
    const add = (from, to, color) => {
      if (!from || !to || from === to) return;
      const key = `${from.i}>${to.i}>${color}`;
      const a = agg.get(key) || { from, to, color, count: 0 };
      a.count++;
      agg.set(key, a);
    };
    for (const e of out) add(selBox, rep(model.byId.get(e.to)), pal.edgeOut);
    for (const e of inc) add(rep(model.byId.get(e.from)), selBox, pal.edgeIn);
    const arcs = [...agg.values()].sort((a, b) => b.count - a.count).slice(0, MAX_ARCS);
    const lit = new Set([selBox]);
    for (const a of arcs) { lit.add(a.from); lit.add(a.to); }
    focus = { lit, arcs, selBox };
  }
  scene.setArcs(focus ? focus.arcs : []);
  scene.setOutline(focus ? focus.selBox : null, pal.select);
  recolor();
  collectLabels();
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
    if (i === state.hovered) c = mix(c, pal.select, 0.25);
    return c;
  });
  scene.setColors(colors);
}

function mix(a, b, t) {
  const pa = parseInt(a.slice(1), 16), pb = parseInt(b.slice(1), 16);
  const ch = s => Math.round(((pa >> s) & 255) * (1 - t) + ((pb >> s) & 255) * t);
  return '#' + ((ch(16) << 16) | (ch(8) << 8) | ch(0)).toString(16).padStart(6, '0');
}

// ---------------------------------------------------------------- labels

function collectLabels() {
  labels = [];
  for (const b of L.boxes) {
    const n = b.node;
    if (b.kind === 'land' && n.kind === 'ecosystem') labels.push({ b, text: n.name, cls: 'island', prio: 0 });
    else if (b.kind === 'terrace' && n.kind === 'dir') labels.push({ b, text: n.name + '/', cls: '', prio: 1 + n.depth });
    else if (b.kind === 'district') labels.push({ b, text: n.name + '/', cls: 'secondary', prio: 2 + n.depth });
  }
  if (focus) {
    for (const a of focus.arcs) {
      for (const b of [a.from, a.to]) {
        if (b.kind === 'building' || b.kind === 'package' || b.kind === 'symbol') labels.push({ b, text: b.node.name, cls: '', prio: -1 });
      }
    }
    labels.push({ b: focus.selBox, text: state.selected.name, cls: 'island', prio: -2 });
  }
  labels.sort((a, b) => a.prio - b.prio);
  scene.requestRender();
}

const labelPool = [];
function drawLabels() {
  const root = $('labels');
  const placed = [];
  let used = 0;
  const seen = new Set();
  for (const l of labels) {
    if (used >= MAX_LABELS) break;
    const key = l.b.node.id + l.cls;
    if (seen.has(key)) continue;
    seen.add(key);
    const b = l.b, top = b.y + b.h;
    const corners = [[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([sx, sz]) => scene.project(b.x + sx * b.w / 2, top, b.z + sz * b.d / 2));
    if (corners.some(c => !c)) continue;
    const xs = corners.map(c => c.x);
    const spread = Math.max(...xs) - Math.min(...xs);
    const width = l.text.length * 6.6 + 14;
    // Region labels need their region to be big enough on screen to be worth naming.
    if (l.prio > 0 && spread < Math.min(width, 90)) continue;
    // Anchor at the top-most corner on screen, i.e. the back edge of the region.
    const anchor = (l.prio > 0 && b.kind !== 'district') ? corners.reduce((a, c) => c.y < a.y ? c : a) : scene.project(b.x, top, b.z);
    const rect = { x: anchor.x - width / 2, y: anchor.y - 20, w: width, h: 18 };
    if (rect.x > root.clientWidth || rect.y > root.clientHeight || rect.x + rect.w < 0 || rect.y + rect.h < 0) continue;
    if (placed.some(p => p.x < rect.x + rect.w && rect.x < p.x + p.w && p.y < rect.y + rect.h && rect.y < p.y + p.h)) continue;
    placed.push(rect);

    let el = labelPool[used];
    if (!el) { el = document.createElement('div'); labelPool.push(el); root.append(el); }
    el.className = 'label ' + l.cls;
    el.textContent = l.text;
    el.style.left = anchor.x + 'px';
    el.style.top = (anchor.y - 2) + 'px';
    el.hidden = false;
    used++;
  }
  for (let i = used; i < labelPool.length; i++) labelPool[i].hidden = true;
}

// ---------------------------------------------------------------- legend & tooltip

function drawLegend() {
  const el = $('legend');
  const parts = [];
  if (state.colorBy === 'language') {
    parts.push('<h3>Language</h3><ul>' + langs.legend().map(e =>
      `<li data-lang="${e.lang === null ? '' : escapeAttr(e.lang)}" data-other="${e.lang === null}"><span class="swatch" style="background:${e.color}"></span>${escapeHTML(e.label)}</li>`).join('') + '</ul>');
  } else {
    parts.push(`<h3>File size</h3><div class="ramp" style="background:linear-gradient(90deg,${pal.seq.join(',')})"></div>
      <div class="ramp-labels"><span>0</span><span>${fmt.format(maxLoc)} lines</span></div>`);
  }
  parts.push(`<h3>Selection</h3><div class="edge-key">
      <span><i class="line" style="background:${pal.edgeOut}"></i>depends on</span>
      <span><i class="line" style="background:${pal.edgeIn}"></i>used by</span></div>`);
  const scaleName = { sqrt: '√ lines of code', linear: 'lines of code', log: 'log lines of code' }[state.heightScale];
  parts.push(`<p>Height: ${scaleName}. Grey blocks are collapsed directories; islands are external dependencies.</p>`);
  el.innerHTML = parts.join('');

  for (const li of el.querySelectorAll('li[data-lang]')) {
    li.addEventListener('mouseenter', () => { state.legendLang = li.dataset.other === 'true' ? null : li.dataset.lang; recolor(); });
    li.addEventListener('mouseleave', () => { state.legendLang = undefined; recolor(); });
  }
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
  tip.innerHTML = html;
  tip.hidden = false;
  const r = tip.parentElement.getBoundingClientRect();
  const tx = Math.min(x - r.left + 14, r.width - tip.offsetWidth - 8);
  const ty = Math.min(y - r.top + 14, r.height - tip.offsetHeight - 8);
  tip.style.left = tx + 'px';
  tip.style.top = ty + 'px';
}

function escapeHTML(s) { return s.replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }
function escapeAttr(s) { return escapeHTML(s); }

// ---------------------------------------------------------------- input

function applyTheme() {
  if (state.theme === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
  pal = readPalette();
  langs = languageColors(model, pal);
  scene.setBackground(pal.water);
  if (L) { scene.setOutline(focus?.selBox, pal.select); refreshFocus(); }
  if (state.selected) panel.show(state.selected); // swatches in the panel
  drawLegend();
}

function bindControls() {
  const map = $('map');
  let down = null, hoverFrame = 0, lastMove = null;

  map.addEventListener('pointerdown', e => { down = { x: e.clientX, y: e.clientY, button: e.button }; map.classList.add('grabbing'); });
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
    const i = scene.pick(e.clientX, e.clientY);
    if (i >= 0) {
      const n = L.boxes[i].node;
      toggle(n);
      select(n.kind === 'symbol' && !state.expanded.has(n.parentNode.id) ? n.parentNode : n);
    }
  });
  map.addEventListener('pointermove', e => {
    lastMove = e;
    if (hoverFrame || (down && e.buttons)) return;
    hoverFrame = requestAnimationFrame(() => {
      hoverFrame = 0;
      const i = scene.pick(lastMove.clientX, lastMove.clientY);
      map.classList.toggle('hovering', i >= 0);
      if (i !== state.hovered) { state.hovered = i; recolor(); }
      showTooltip(i, lastMove.clientX, lastMove.clientY);
    });
  });
  map.addEventListener('pointerleave', () => { state.hovered = -1; showTooltip(-1); recolor(); });
  map.addEventListener('contextmenu', e => e.preventDefault());

  $('expand-level').onclick = () => setLevel(state.level + 1);
  $('collapse-level').onclick = () => setLevel(state.level - 1);
  $('fit').onclick = () => scene.fit(L.bounds);
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
  const std = $('show-std');
  std.checked = state.showStd;
  std.onchange = () => { state.showStd = std.checked; relayout(); };
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => state.theme === 'auto' && applyTheme());

  window.addEventListener('keydown', e => {
    if (e.target.closest('input, select, textarea, dialog') || e.ctrlKey || e.metaKey || e.altKey) return;
    const sel = state.selected;
    switch (e.key) {
      case 'Escape': select(null); break;
      case 'f': case 'F': scene.fit(L.bounds); break;
      case 'q': case 'Q': scene.setIso(scene.quarter - 1); break;
      case 'e': case 'E': scene.setIso(scene.quarter + 1); break;
      case '+': case '=': setLevel(state.level + 1); break;
      case '-': case '_': setLevel(state.level - 1); break;
      case '?': $('help').showModal(); break;
      case 'Enter': if (sel) { toggle(sel); select(sel); } break;
      case 'Backspace': if (sel?.parentNode) reveal(sel.parentNode); break;
      default: return;
    }
    e.preventDefault();
  });
}

main().catch(err => {
  $('status').textContent = `Failed to load: ${err.message}`;
  console.error(err);
});
