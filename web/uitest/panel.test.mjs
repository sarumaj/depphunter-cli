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
  addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback); }
  insertAt(c, at) {
    if (c instanceof El) { c.remove(); c.parentElement = this; } else c = String(c);
    this.childNodes.splice(at ?? this.childNodes.length, 0, c);
  }
  append(...children) { for (const c of children) this.insertAt(c); }
  after(el) {
    el.remove?.();
    const p = this.parentElement;
    p.insertAt(el, p.childNodes.indexOf(this) + 1);
  }
  replaceWith(el) {
    const p = this.parentElement;
    if (!p) return;
    const at = p.childNodes.indexOf(this);
    this.remove();
    p.insertAt(el, at);
  }
  remove() {
    const p = this.parentElement;
    if (!p) return;
    p.childNodes.splice(p.childNodes.indexOf(this), 1);
    this.parentElement = null;
  }
  replaceChildren(...children) {
    for (const c of this.children) c.parentElement = null;
    this.childNodes = [];
    this.append(...children);
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
  const [tag, className] = sel.split('.');
  return el => (!tag || el.tagName === tag.toUpperCase()) && (!className || el.classList.contains(className));
}

/** Delivers an event to an element's own listeners; nothing here relies on bubbling. */
function fire(el, type, extra = {}) {
  const ev = { type, key: extra.key, preventDefault() {}, stopPropagation() {}, ...extra };
  for (const callback of el.listeners[type] || []) callback(ev);
}

globalThis.document.createElement = tag => new El(tag);
// CSS.escape only has to round-trip through the selector matcher above.
globalThis.CSS ??= { escape: s => s };

// Every request the page could make, counted. The trees must never add to it.
let requests = 0;
globalThis.fetch = async () => { requests++; throw new Error('no network in a test'); };

const { buildModel } = await import('../static/model.js');
const { Panel, hexDump, mediaKind, findMatches, findBar } = await import('../static/panel.js');

const directory = (path, parent) => ({ id: `d:${path}`, kind: 'dir', name: path.split('/').pop(), path, parent });
const file = (path, parent, language, loc) => ({ id: `f:${path}`, kind: 'file', name: path.split('/').pop(), path, parent, lang: language, loc });
const packageNode = (ecosystem, name, extra) => ({ id: `p:${ecosystem}:${name}`, kind: 'package', name, parent: `e:${ecosystem}`, ...extra });

// A slice of this repository: the Go analyzer, which imports go/parser, is imported
// by its parent package and by the command; and a lock file's worth of npm packages,
// with a chain four deep and a pair that depend on each other.
const GRAPH = {
  nodes: [
    { id: 'd:.', kind: 'dir', name: '.', path: '.' },
    directory('cmd', 'd:.'), directory('internal', 'd:.'), directory('internal/lang', 'd:internal'),
    directory('internal/lang/golang', 'd:internal/lang'), directory('web', 'd:.'),
    file('cmd/main.go', 'd:cmd', 'Go', 40),
    file('internal/lang/lang.go', 'd:internal/lang', 'Go', 20),
    file('internal/lang/golang/golang.go', 'd:internal/lang/golang', 'Go', 300),
    file('internal/lang/golang/golang_test.go', 'd:internal/lang/golang', 'Go', 100),
    file('internal/lang/golang/testdata.yaml', 'd:internal/lang/golang', 'YAML', 100),
    file('web/app.ts', 'd:web', 'TypeScript', 80),
    { id: 'e:gostd', kind: 'ecosystem', name: 'go std' },
    packageNode('gostd', 'go/parser'),
    { id: 'e:npm', kind: 'ecosystem', name: 'npm' },
    packageNode('npm', 'a', { version: '1.2.3', requested: '^1.2', index: 'https://registry.npmjs.org/' }),
    packageNode('npm', 'b', { version: '0.21.5', platform: 'os=linux & cpu=x64' }), packageNode('npm', 'c'), packageNode('npm', 'd'), packageNode('npm', 'x'), packageNode('npm', 'y'),
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
    colorOf: language => `color(${language})`,
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
    const dependencies = s.get('Depends on');
    assert.deepEqual(outline(dependencies), ['a', 'x']);
    const before = requests;
    fire(named(dependencies, 'a').querySelector('.twisty'), 'click');
    fire(named(dependencies, 'b').querySelector('.twisty'), 'click');
    fire(named(dependencies, 'c').querySelector('.twisty'), 'click');
    assert.deepEqual(outline(dependencies), ['a', '  b', '    c', '      d', 'x']);
    assert.equal(requests, before, 'opening a row asked the server for something');
    // Opening is not selecting: the twisty is a control of its own.
    assert.deepEqual(selected, []);
    // A leaf has nothing to open.
    assert.equal(named(dependencies, 'd').querySelector('.twisty').tagName, 'SPAN');
  });

  // Verifies: REQ-MAP-047
  it('opens on the right arrow and closes, with everything below, on the left', () => {
    const dependencies = show('f:web/app.ts').get('Depends on');
    const a = named(dependencies, 'a');
    assert.equal(a.getAttribute('aria-expanded'), 'false');
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.equal(a.getAttribute('aria-expanded'), 'true');
    fire(named(dependencies, 'b'), 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(dependencies), ['a', '  b', '    c', 'x']);
    // A second right arrow on an open row does not close it.
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.equal(a.getAttribute('aria-expanded'), 'true');

    fire(a, 'keydown', { key: 'ArrowLeft' });
    assert.deepEqual(outline(dependencies), ['a', 'x'], 'closing a row left its branch behind');
    assert.equal(a.getAttribute('aria-expanded'), 'false');
    // What was open below it went with it: opening a again shows b closed.
    fire(a, 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(dependencies), ['a', '  b', 'x']);
    assert.deepEqual(selected, [], 'an arrow key selected the row');
  });

  // Verifies: REQ-MAP-048
  it('shows a cycle once more, marked, and does not open it', () => {
    const dependencies = show('f:web/app.ts').get('Depends on');
    fire(named(dependencies, 'x').querySelector('.twisty'), 'click');
    fire(named(dependencies, 'y').querySelector('.twisty'), 'click');
    assert.deepEqual(outline(dependencies), ['a', 'x', '  y', '    x']);
    const repeat = rows(dependencies).at(-1);
    assert.equal(repeat.querySelector('.twisty').tagName, 'SPAN', 'the repeat can be opened');
    assert.equal(repeat.querySelector('.twisty').textContent, '↻');
    assert.match(repeat.getAttribute('title'), /already open further up/);
    fire(repeat, 'keydown', { key: 'ArrowRight' });
    assert.deepEqual(outline(dependencies), ['a', 'x', '  y', '    x'], 'the repeat opened');
  });

  // Verifies: REQ-MAP-046
  it('reopens what was open when a live update redraws the panel', () => {
    let dependencies = show('f:web/app.ts').get('Depends on');
    fire(named(dependencies, 'a').querySelector('.twisty'), 'click');
    fire(named(dependencies, 'b').querySelector('.twisty'), 'click');
    const open = outline(dependencies);
    assert.deepEqual(open, ['a', '  b', '    c', 'x']);

    // What app.js does on an update: a new model out of the new graph, and the same
    // node shown again from it.
    model = buildModel(GRAPH);
    panel.model = model;
    dependencies = show('f:web/app.ts', true).get('Depends on');
    assert.deepEqual(outline(dependencies), open, 'the redraw closed a branch the reader had open');
    assert.equal(named(dependencies, 'b').getAttribute('aria-expanded'), 'true');
    assert.equal(named(dependencies, 'c').getAttribute('aria-expanded'), 'false');
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

  // Verifies: REQ-JS-018
  it('says which platforms a platform binary installs on', () => {
    show('p:npm:b');
    assert.deepEqual(stats(), {
      version: '0.21.5', 'installs on': 'os=linux & cpu=x64', 'importing files': '0', ecosystem: 'npm',
    });
  });
});

describe('a file that is not text', () => {
  // Verifies: REQ-MAP-062
  it('previews pictures, clips and recordings, by extension', () => {
    assert.equal(mediaKind('docs/screenshots/shot1.png'), 'image');
    assert.equal(mediaKind('LOGO.JPEG'), 'image', 'an upper-case extension is not previewed');
    assert.equal(mediaKind('demo/tour.webm'), 'video');
    assert.equal(mediaKind('sounds/ping.mp3'), 'audio');
    // An SVG is text and shown as source; the rest are bytes.
    for (const p of ['icon.svg', 'app.wasm', 'vendor/archive.zip', 'Makefile', 'dir.png/file']) {
      assert.equal(mediaKind(p), null, `${p} is previewed`);
    }
  });

  // Verifies: REQ-MAP-062
  it('shows bytes the way hexdump -C does', () => {
    const bytes = new Uint8Array([...'PK\x03\x04hello, world!\x00\x01'].map(c => c.charCodeAt(0)));
    assert.deepEqual(hexDump(bytes), [
      '00000000  50 4b 03 04 68 65 6c 6c  6f 2c 20 77 6f 72 6c 64  |PK..hello, world|',
      '00000010  21 00 01                                          |!..|',
    ]);
    assert.deepEqual(hexDump(new Uint8Array()), []);
  });
});

describe('the details maximized', () => {
  /** A panel with its maximize button, and the browser storage it remembers the choice in. */
  const setup = () => {
    const store = new Map();
    globalThis.localStorage = { getItem: k => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)) };
    const make = () => {
      const shell = new El('main'), root = new El('aside'), body = new El('div'), button = new El('button');
      shell.append(root);
      root.append(button, body);
      const find = root.querySelector.bind(root);
      root.querySelector = sel => (sel === '#panel-max' ? button : find(sel));
      const panel = new Panel(root, body, { model, colorOf: () => '', onSelect() {}, linkKind: () => 'import', historyOf: () => null });
      return { panel, root, button };
    };
    return { make, store };
  };

  // Verifies: REQ-UI-015
  it('toggles over the map from its button, and says which it will do', () => {
    const { make } = setup();
    try {
      const { root, button } = make();
      assert.equal(root.classList.contains('max'), false, 'a first panel opened maximized');
      assert.equal(button.getAttribute('aria-pressed'), 'false');
      assert.match(button.getAttribute('title'), /^Maximize/);
      button.onclick();
      assert.equal(root.classList.contains('max'), true, 'the button did not maximize it');
      assert.equal(button.getAttribute('aria-pressed'), 'true');
      assert.match(button.getAttribute('title'), /^Restore/);
      button.onclick();
      assert.equal(root.classList.contains('max'), false, 'the button did not restore it');
    } finally {
      delete globalThis.localStorage;
    }
  });

  // Verifies: REQ-UI-015
  it('opens the next time the way it was left', () => {
    const { make } = setup();
    try {
      make().button.onclick();
      assert.equal(make().root.classList.contains('max'), true, 'a maximized panel came back beside the map');
      const again = make();
      again.button.onclick();
      assert.equal(make().root.classList.contains('max'), false, 'a restored panel came back maximized');
    } finally {
      delete globalThis.localStorage;
    }
  });
});

describe('finding in a file', () => {
  // Verifies: REQ-MAP-063
  it('finds every place the text occurs, ignoring case and taking it literally', () => {
    const lines = ['func Main() {', '  main()  // MAIN', 'a.b(c)', ''];
    assert.deepEqual(findMatches(lines, 'main'), [
      { line: 0, start: 5, end: 9 }, { line: 1, start: 2, end: 6 }, { line: 1, start: 13, end: 17 },
    ]);
    assert.deepEqual(findMatches(lines, 'a.b('), [{ line: 2, start: 0, end: 4 }], 'a dot or a bracket was read as a pattern');
    assert.deepEqual(findMatches(['aaaa'], 'aa'), [{ line: 0, start: 0, end: 2 }, { line: 0, start: 2, end: 4 }], 'overlaps counted twice');
    assert.deepEqual(findMatches(lines, ''), []);
    assert.equal(findMatches(Array(10).fill('x x x'), 'x', 7).length, 7, 'the limit was not kept');
  });

  /** A painted source pane: one line element per line, the way paint() leaves it. */
  const pane = lines => {
    const pre = new El('pre');
    pre.dataset.filled = '1';
    for (const l of lines) { const lineElement = new El('span'); lineElement.className = 'ln'; lineElement.append(l); pre.append(lineElement); }
    return pre;
  };
  const type = (bar, text) => { bar.input.value = text; fire(bar.input, 'input'); };
  const count = bar => bar.el.querySelector('span.p-find-count').textContent;
  const current = pre => pre.children.findIndex(el => el.classList.contains('current'));

  // Verifies: REQ-MAP-063
  it('counts the matches, marks their lines, and steps through them both ways', () => {
    const pre = pane(['alpha', 'beta', 'alphabet', 'gamma']);
    let kept = null;
    const bar = findBar(pre, '', q => { kept = q; });
    type(bar, 'ALPHA');
    assert.equal(kept, 'ALPHA', 'what was looked for was not remembered');
    assert.equal(count(bar), '1 of 2');
    assert.deepEqual(pre.children.map(el => el.classList.contains('found')), [true, false, true, false]);
    assert.equal(current(pre), 0);
    fire(bar.input, 'keydown', { key: 'Enter' });
    assert.equal(count(bar), '2 of 2');
    assert.equal(current(pre), 2);
    fire(bar.input, 'keydown', { key: 'Enter' });
    assert.equal(current(pre), 0, 'stepping on from the last did not wrap round');
    fire(bar.input, 'keydown', { key: 'Enter', shiftKey: true });
    assert.equal(current(pre), 2, 'Shift+Enter did not step back');
    type(bar, 'delta');
    assert.equal(count(bar), 'No matches');
    assert.ok(pre.children.every(el => !el.classList.contains('found')), 'a line stayed marked');
  });

  // Verifies: REQ-MAP-063
  it('clears with Escape before the panel does, and finds again when the text comes in', () => {
    const pre = pane(['one', 'two']);
    const bar = findBar(pre, 'two');
    bar.run(false);
    assert.equal(count(bar), '1 of 1', 'what was being looked for was not found again');
    let stopped = false;
    fire(bar.input, 'keydown', { key: 'Escape', stopPropagation: () => { stopped = true; } });
    assert.equal(bar.input.value, '');
    assert.ok(stopped, 'Escape went on to close the panel as well');
    assert.equal(count(bar), '');
    // Before the text arrives there is nothing to find in the placeholder.
    const loading = pane(['Loading…']);
    delete loading.dataset.filled;
    const early = findBar(loading, 'Loading');
    early.run();
    assert.equal(count(early), '');
  });
});

describe('opening the file from the corner', () => {
  /** A panel with its corner: the open slot, maximize and close, and a width to report. */
  const cornered = (openLabel = 'Open in VS Code') => {
    const shell = new El('main'), root = new El('aside'), body = new El('div');
    const tools = new El('div'), slot = new El('span');
    tools.append(slot);
    tools.offsetWidth = 150;
    shell.append(root);
    root.append(tools, body);
    const find = root.querySelector.bind(root);
    root.querySelector = sel => ({ '.p-tools': tools, '#panel-open': slot }[sel] ?? find(sel));
    root.style.setProperty = (k, v) => { root.style[k] = v; };
    const opened = [];
    const panel = new Panel(root, body, {
      model, colorOf: () => '', onSelect() {}, linkKind: () => 'import', historyOf: () => null,
      openLabel, onOpen: (path, line) => opened.push([path, line]),
    });
    return { panel, root, body, slot, opened };
  };

  // Verifies: REQ-UI-016
  it('pins the button beside maximize and close, short, and opens the file', () => {
    const { panel, root, body, slot, opened } = cornered();
    panel.show(model.byId.get('f:cmd/main.go'));
    const [button] = slot.children;
    assert.ok(button, 'no open button in the corner');
    assert.equal(button.textContent, 'VS Code ↗');
    assert.equal(button.getAttribute('title'), 'Open in VS Code (O)', 'the full action is not in its title');
    assert.equal(body.querySelector('button.p-open'), null, 'the button is also in the scrolling content');
    assert.equal(root.style['--p-tools'], '158px', 'what shares the top of the panel was not told how wide the corner is');
    fire(button, 'click');
    assert.deepEqual(opened, [['cmd/main.go', 1]]);
  });

  // Verifies: REQ-UI-016
  it('empties the corner for what is not a file, and where there is no editor', () => {
    const { panel, slot } = cornered();
    panel.show(model.byId.get('f:cmd/main.go'));
    panel.show(model.byId.get('d:cmd'));
    assert.equal(slot.children.length, 0, 'a directory kept the last file\'s open button');
    const none = cornered(null);
    none.panel.show(model.byId.get('f:cmd/main.go'));
    assert.equal(none.slot.children.length, 0, 'a static export offered to open the file');
  });
});

describe('a binary file in a hex editor', () => {
  // Verifies: REQ-EXT-034
  it('offers a hex editor for a file found to have no text, and the editor again after', async () => {
    const shell = new El('main'), root = new El('aside'), body = new El('div'), tools = new El('div'), slot = new El('span');
    tools.append(slot);
    shell.append(root);
    root.append(tools, body);
    const find = root.querySelector.bind(root);
    root.querySelector = sel => ({ '.p-tools': tools, '#panel-open': slot }[sel] ?? find(sel));
    const opened = [];
    const p = new Panel(root, body, {
      model, colorOf: () => '', onSelect() {}, linkKind: () => 'import', historyOf: () => null,
      openLabel: 'Open in VS Code', onOpen: (...a) => opened.push(a),
    });
    const real = globalThis.fetch;
    globalThis.fetch = async () => ({ status: 415, ok: false, headers: { get: () => 'application/octet-stream' }, text: async () => 'binary file' });
    try {
      p.show(model.byId.get('f:internal/lang/golang/testdata.yaml'));
      await new Promise(r => setTimeout(r, 0));
      const [hex] = slot.children;
      assert.equal(hex?.textContent, 'Hex editor ↗', 'a binary file was offered to the editor as text');
      assert.equal(p.hexFor('internal/lang/golang/testdata.yaml'), true);
      fire(hex, 'click');
      assert.deepEqual(opened.at(-1), ['internal/lang/golang/testdata.yaml', 1, true]);
      // Another file is not known to be binary until it is read.
      globalThis.fetch = real;
      p.show(model.byId.get('f:cmd/main.go'));
      assert.equal(slot.children[0]?.textContent, 'VS Code ↗');
      assert.equal(p.hexFor('internal/lang/golang/testdata.yaml'), false, 'the last binary file was still taken for binary');
    } finally {
      globalThis.fetch = real;
    }
  });

  // Verifies: REQ-EXT-034
  it('does not offer a hex editor for a picture, which an editor shows as itself', () => {
    const p = Object.create(Panel.prototype);
    p.binaryPath = 'docs/logo.png';
    assert.equal(p.hexFor('docs/logo.png'), false);
    p.binaryPath = 'bin/tool.wasm';
    assert.equal(p.hexFor('bin/tool.wasm'), true);
  });
});
