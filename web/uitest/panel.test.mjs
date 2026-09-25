// The side panel's lists of what a node depends on and what depends on it.
//
// They are trees built out of the edges the model already holds, opened a row at a
// time, and everything that can go wrong with them is about which rows exist and in
// what order: a branch that opens into nothing, a cycle that opens for ever, a live
// update that closes what the reader had open. None of that needs a browser - only
// enough of a document for the panel to build its rows in, which is what `El` below
// is. It keeps the tree of elements and the listeners, and nothing about layout.

import assert from 'node:assert/strict';
import { describe, it, beforeEach } from 'node:test';

import './stub.mjs';

/** Just enough of an element for panel.js to build, insert, find and remove rows. */
class El {
  constructor(tag) {
    Object.assign(this, {
      tagName: tag.toUpperCase(), childNodes: [], parentElement: null, attributes: new Map(),
      dataset: {}, style: {}, className: '', listeners: {}, hidden: false, scrollTop: 0,
    });
  }
  get children() { return this.childNodes.filter(c => c instanceof El); }
  get lastChild() { return this.childNodes.at(-1) ?? null; }
  get nextElementSibling() {
    const sibs = this.parentElement?.children || [];
    return sibs[sibs.indexOf(this) + 1] ?? null;
  }
  get classList() {
    const names = () => this.className.split(/\s+/).filter(Boolean);
    const set = list => { this.className = list.join(' '); };
    return {
      contains: c => names().includes(c),
      add: c => { if (!names().includes(c)) set([...names(), c]); },
      remove: c => set(names().filter(n => n !== c)),
      toggle: (c, on = !names().includes(c)) => (on ? set([...new Set([...names(), c])]) : set(names().filter(n => n !== c))),
    };
  }
  setAttribute(k, v) { this.attributes.set(k, String(v)); if (k === 'hidden') this.hidden = true; }
  getAttribute(k) { return this.attributes.get(k) ?? null; }
  addEventListener(type, fn) { (this.listeners[type] ||= []).push(fn); }
  insertAt(c, at) {
    if (c instanceof El) { c.remove(); c.parentElement = this; } else c = String(c);
    this.childNodes.splice(at ?? this.childNodes.length, 0, c);
  }
  append(...cs) { for (const c of cs) this.insertAt(c); }
  after(el) {
    el.remove?.();
    const p = this.parentElement;
    p.insertAt(el, p.childNodes.indexOf(this) + 1);
  }
  remove() {
    const p = this.parentElement;
    if (!p) return;
    p.childNodes.splice(p.childNodes.indexOf(this), 1);
    this.parentElement = null;
  }
  replaceChildren(...cs) {
    for (const c of this.children) c.parentElement = null;
    this.childNodes = [];
    this.append(...cs);
  }
  get textContent() { return this.childNodes.map(c => (c instanceof El ? c.textContent : c)).join(''); }
  set textContent(v) { this.replaceChildren(String(v)); }
  querySelectorAll(sel) {
    const want = matcher(sel), out = [];
    const walk = el => { for (const c of el.children) { if (want(c)) out.push(c); walk(c); } };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] ?? null; }
  scrollIntoView() {}
  focus() {}
}

/** The selectors panel.js and these tests use: `tag`, `.class`, `tag.class`, `[data-x="v"]`. */
function matcher(sel) {
  const data = /^\[data-([a-z]+)="(.*)"\]$/.exec(sel);
  if (data) return el => el.dataset[data[1]] === data[2];
  const [tag, cls] = sel.split('.');
  return el => (!tag || el.tagName === tag.toUpperCase()) && (!cls || el.classList.contains(cls));
}

/** Delivers an event to an element's own listeners; nothing here relies on bubbling. */
function fire(el, type, extra = {}) {
  const ev = { type, key: extra.key, preventDefault() {}, stopPropagation() {}, ...extra };
  for (const fn of el.listeners[type] || []) fn(ev);
}

globalThis.document.createElement = tag => new El(tag);
// CSS.escape only has to round-trip through the selector matcher above.
globalThis.CSS ??= { escape: s => s };

// Every request the page could make, counted. The trees must never add to it.
let requests = 0;
globalThis.fetch = async () => { requests++; throw new Error('no network in a test'); };

const { buildModel } = await import('../static/model.js');
const { Panel } = await import('../static/panel.js');

const dir = (path, parent) => ({ id: `d:${path}`, kind: 'dir', name: path.split('/').pop(), path, parent });
const file = (path, parent, lang, loc) => ({ id: `f:${path}`, kind: 'file', name: path.split('/').pop(), path, parent, lang, loc });
const pkg = (eco, name, extra) => ({ id: `p:${eco}:${name}`, kind: 'package', name, parent: `e:${eco}`, ...extra });

// A slice of this repository: the Go analyzer, which imports go/parser, is imported
// by its parent package and by the command; and a lock file's worth of npm packages,
// with a chain four deep and a pair that depend on each other.
const GRAPH = {
  nodes: [
    { id: 'd:.', kind: 'dir', name: '.', path: '.' },
    dir('cmd', 'd:.'), dir('internal', 'd:.'), dir('internal/lang', 'd:internal'),
    dir('internal/lang/golang', 'd:internal/lang'), dir('web', 'd:.'),
    file('cmd/main.go', 'd:cmd', 'Go', 40),
    file('internal/lang/lang.go', 'd:internal/lang', 'Go', 20),
    file('internal/lang/golang/golang.go', 'd:internal/lang/golang', 'Go', 300),
    file('internal/lang/golang/golang_test.go', 'd:internal/lang/golang', 'Go', 100),
    file('internal/lang/golang/testdata.yaml', 'd:internal/lang/golang', 'YAML', 100),
    file('web/app.ts', 'd:web', 'TypeScript', 80),
    { id: 'e:gostd', kind: 'ecosystem', name: 'go std' },
    pkg('gostd', 'go/parser'),
    { id: 'e:npm', kind: 'ecosystem', name: 'npm' },
    pkg('npm', 'a', { version: '1.2.3', requested: '^1.2', index: 'https://registry.npmjs.org/' }),
    pkg('npm', 'b'), pkg('npm', 'c'), pkg('npm', 'd'), pkg('npm', 'x'), pkg('npm', 'y'),
  ],
  edges: [
    { from: 'f:internal/lang/golang/golang.go', to: 'p:gostd:go/parser', kind: 'import' },
    { from: 'f:cmd/main.go', to: 'd:internal/lang/golang', kind: 'import' },
    { from: 'f:internal/lang/lang.go', to: 'd:internal/lang/golang', kind: 'import' },
    { from: 'f:web/app.ts', to: 'p:npm:a', kind: 'import' },
    { from: 'f:web/app.ts', to: 'p:npm:x', kind: 'import' },
    { from: 'p:npm:a', to: 'p:npm:b', kind: 'depends' },
    { from: 'p:npm:b', to: 'p:npm:c', kind: 'depends' },
    { from: 'p:npm:c', to: 'p:npm:d', kind: 'depends' },
    { from: 'p:npm:x', to: 'p:npm:y', kind: 'depends' },
    { from: 'p:npm:y', to: 'p:npm:x', kind: 'depends' },
  ],
};

let model, panel, selected;

/** A panel as app.js makes one, with the hooks it is given reduced to a record. */
function makePanel() {
  model = buildModel(GRAPH);
  selected = [];
  const shell = new El('main'), root = new El('aside'), body = new El('div');
  shell.append(root);
  root.append(body);
  panel = new Panel(root, body, {
    model,
    colorOf: lang => `color(${lang})`,
    onSelect: n => selected.push(n),
    linkKind: () => 'import',
    historyOf: () => null,
  });
}

/** Shows a node and returns its sections by heading. */
function show(id, keepScroll = false) {
  panel.show(model.byId.get(id), keepScroll);
  const sections = new Map();
  for (const s of panel.body.children) {
    const h4 = s.children.find(c => c.tagName === 'H4');
    if (h4) sections.set(h4.childNodes.filter(c => typeof c === 'string').join('').trim(), s);
  }
  return sections;
}

/** The rows of a tree section, as their names with two spaces per level of depth. */
const rows = section => section.querySelectorAll('li.tree-row');
const named = (section, name) => rows(section).find(r => r.querySelector('.name').textContent === name);
const outline = section => rows(section).map(r => r.dataset.branch.split('>').length - 2)
  .map((depth, i) => '  '.repeat(depth) + rows(section)[i].querySelector('.name').textContent);

describe('the dependency lists', () => {
  beforeEach(makePanel);

  // Verifies: REQ-MAP-029
  it('lists what a package imports and who imports it, and selects a row when it is used', () => {
    const s = show('d:internal/lang/golang');
    assert.deepEqual(outline(s.get('Depends on')), ['go/parser']);
    assert.deepEqual(outline(s.get('Used by')).sort(), ['cmd/main.go', 'internal/lang/lang.go']);

    // A row is a way to that node: clicked, or Enter on it from the keyboard.
    fire(named(s.get('Used by'), 'cmd/main.go'), 'click');
    fire(named(s.get('Depends on'), 'go/parser'), 'keydown', { key: 'Enter' });
    assert.deepEqual(selected.map(n => n.id), ['f:cmd/main.go', 'p:gostd:go/parser']);
  });

  // Verifies: REQ-MAP-045
  it('opens a row into its own dependencies, from the model and nothing else', () => {
    const s = show('f:web/app.ts');
    const deps = s.get('Depends on');
    assert.deepEqual(outline(deps), ['a', 'x']);
    const before = requests;
    fire(named(deps, 'a').querySelector('.twisty'), 'click');
    fire(named(deps, 'b').querySelector('.twisty'), 'click');
    fire(named(deps, 'c').querySelector('.twisty'), 'click');
    assert.deepEqual(outline(deps), ['a', '  b', '    c', '      d', 'x']);
    assert.equal(requests, before, 'opening a row asked the server for something');
    // Opening is not selecting: the twisty is a control of its own.
    assert.deepEqual(selected, []);
    // A leaf has nothing to open.
    assert.equal(named(deps, 'd').querySelector('.twisty').tagName, 'SPAN');
  });

  // Verifies: REQ-MAP-047
  it('opens on the right arrow and closes, with everything below, on the left', () => {
    const deps = show('f:web/app.ts').get('Depends on');
    const a = named(deps, 'a');
    assert.equal(a.getAttribute('aria-expanded'), 'false');
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.equal(a.getAttribute('aria-expanded'), 'true');
    fire(named(deps, 'b'), 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(deps), ['a', '  b', '    c', 'x']);
    // A second right arrow on an open row does not close it.
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.equal(a.getAttribute('aria-expanded'), 'true');

    fire(a, 'keydown', { key: 'ArrowLeft' });
    assert.deepEqual(outline(deps), ['a', 'x'], 'closing a row left its branch behind');
    assert.equal(a.getAttribute('aria-expanded'), 'false');
    // What was open below it went with it: opening a again shows b closed.
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(deps), ['a', '  b', 'x']);
    assert.deepEqual(selected, [], 'an arrow key selected the row');
  });

  // Verifies: REQ-MAP-048
  it('shows a cycle once more, marked, and does not open it', () => {
    const deps = show('f:web/app.ts').get('Depends on');
    fire(named(deps, 'x').querySelector('.twisty'), 'click');
    fire(named(deps, 'y').querySelector('.twisty'), 'click');
    assert.deepEqual(outline(deps), ['a', 'x', '  y', '    x']);
    const repeat = rows(deps).at(-1);
    assert.equal(repeat.querySelector('.twisty').tagName, 'SPAN', 'the repeat can be opened');
    assert.equal(repeat.querySelector('.twisty').textContent, '↻');
    assert.match(repeat.getAttribute('title'), /already open further up/);
    fire(repeat, 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(deps), ['a', 'x', '  y', '    x'], 'the repeat opened');
  });

  // Verifies: REQ-MAP-046
  it('reopens what was open when a live update redraws the panel', () => {
    let deps = show('f:web/app.ts').get('Depends on');
    fire(named(deps, 'a').querySelector('.twisty'), 'click');
    fire(named(deps, 'b').querySelector('.twisty'), 'click');
    const open = outline(deps);
    assert.deepEqual(open, ['a', '  b', '    c', 'x']);

    // What app.js does on an update: a new model out of the new graph, and the same
    // node shown again from it.
    model = buildModel(GRAPH);
    panel.model = model;
    deps = show('f:web/app.ts', true).get('Depends on');
    assert.deepEqual(outline(deps), open, 'the redraw closed a branch the reader had open');
    assert.equal(named(deps, 'b').getAttribute('aria-expanded'), 'true');
    assert.equal(named(deps, 'c').getAttribute('aria-expanded'), 'false');
  });
});

describe('the numbers for a node with no source', () => {
  beforeEach(makePanel);

  /** The stat tiles, as {label: value}. */
  const stats = () => Object.fromEntries(panel.body.querySelector('.stats').children
    .map(t => [t.querySelector('.k').textContent, t.querySelector('.v').textContent]));

  // Verifies: REQ-MAP-030
  it('gives a directory its files, lines, sub-directories and language mix', () => {
    const s = show('d:internal/lang');
    assert.deepEqual(stats(), { files: '4', lines: '520', 'sub-directories': '1' });
    const mix = s.get('Languages').querySelector('.bar-legend').children.map(c => c.textContent);
    assert.deepEqual(mix, ['Go 81%', 'YAML 19%']);
  });

  // Verifies: REQ-MAP-030
  it('gives a package its version, importers, ecosystem and index', () => {
    show('p:npm:a');
    assert.deepEqual(stats(), {
      version: '1.2.3', requested: '^1.2', 'importing files': '1', ecosystem: 'npm', index: 'registry.npmjs.org',
    });
    show('e:npm');
    assert.deepEqual(stats(), { packages: '6' });
  });
});
