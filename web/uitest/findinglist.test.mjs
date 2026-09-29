// The findings list: every finding the map shows, and catching one from there.
//
// What can go wrong with it is what it is for. A list in another order than the pins
// and the panel is a list nobody trusts; one that shows what the filters took off the
// map is a list of buildings that are not there; and a catch from the list that is
// not the street's catch - another entry, another building, a bug still walking - is
// two backpacks pretending to be one. So the order, the filters and the catch are
// checked here against the modules the map itself uses: the model, the filters, the
// findings index, the bugs and the backpack.

import assert from 'node:assert/strict';
import { describe, it, beforeEach } from 'node:test';

import { box, scene } from './stub.mjs';

/** Just enough of an element for the list to be built, searched, clicked and focused. */
class El {
  constructor(tag) {
    Object.assign(this, {
      tagName: tag.toUpperCase(), childNodes: [], parentElement: null, attributes: new Map(),
      dataset: {}, style: {}, className: '', listeners: {}, hidden: false,
    });
  }
  get children() { return this.childNodes.filter(c => c instanceof El); }
  get firstElementChild() { return this.children[0] ?? null; }
  get classList() {
    return { contains: c => this.className.split(/\s+/).includes(c) };
  }
  setAttribute(k, v) { this.attributes.set(k, String(v)); }
  getAttribute(k) { return this.attributes.get(k) ?? null; }
  addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback); }
  append(...children) {
    for (const c of children) {
      if (c instanceof El) c.parentElement = this;
      this.childNodes.push(c instanceof El ? c : String(c));
    }
  }
  replaceChildren(...children) {
    this.childNodes = [];
    this.append(...children);
  }
  contains(element) {
    for (let p = element; p; p = p.parentElement) if (p === this) return true;
    return false;
  }
  closest(sel) {
    for (let p = this; p; p = p.parentElement) if (p.tagName === sel.toUpperCase()) return p;
    return null;
  }
  get textContent() { return this.childNodes.map(c => (c instanceof El ? c.textContent : c)).join(''); }
  querySelector(sel) {
    const className = sel.replace(/^\./, '');
    const walk = element => {
      for (const c of element.children) {
        if (c.classList.contains(className)) return c;
        const deeper = walk(c);
        if (deeper) return deeper;
      }
      return null;
    };
    return walk(this);
  }
  focus() { globalThis.document.activeElement = this; }
}

/** Delivers an event to an element's own listeners; nothing here relies on bubbling. */
function fire(element, type, extra = {}) {
  let stopped = false;
  const event = { type, target: element, preventDefault() {}, stopPropagation() { stopped = true; }, ...extra };
  for (const callback of element.listeners[type] || []) callback(event);
  return { stopped };
}

globalThis.document.createElement = tag => new El(tag);

const { buildModel } = await import('../static/model.js');
const { computeVisibility } = await import('../static/filter.js');
const { indexFindings } = await import('../static/findings.js');
const { Backpack, catchFinding } = await import('../static/backpack.js');
const { Bugs } = await import('../static/bugs.js');
const { FindingList, findingRows } = await import('../static/findinglist.js');

const directory = (path, parent) => ({ id: `d:${path}`, kind: 'dir', name: path.split('/').pop(), path, parent });
const file = (path, parent, language, loc) => ({ id: `f:${path}`, kind: 'file', name: path.split('/').pop(), path, parent, lang: language, loc });
const packageNode = (ecosystem, name) => ({ id: `p:${ecosystem}:${name}`, kind: 'package', name, parent: `e:${ecosystem}` });

// Go in src/, TypeScript in web/; npm is used only by the TypeScript.
const model = buildModel({
  nodes: [
    { id: 'd:.', kind: 'dir', name: '.', path: '.' },
    directory('src', 'd:.'), directory('web', 'd:.'),
    file('src/main.go', 'd:src', 'Go', 500),
    file('src/util.go', 'd:src', 'Go', 80),
    file('web/app.ts', 'd:web', 'TypeScript', 300),
    { id: 'e:go', kind: 'ecosystem', name: 'go' }, packageNode('go', 'golang.org/x/net'),
    { id: 'e:npm', kind: 'ecosystem', name: 'npm' }, packageNode('npm', 'lodash'), packageNode('npm', 'axios'),
  ],
  edges: [
    { from: 'f:src/main.go', to: 'p:go:golang.org/x/net', kind: 'import' },
    { from: 'f:web/app.ts', to: 'p:npm:lodash', kind: 'import' },
    { from: 'f:web/app.ts', to: 'p:npm:axios', kind: 'import' },
  ],
});

// Listed deliberately out of order: the list has to put them in it.
const FINDINGS = [
  { id: 'lint-1', severity: 'low', title: 'unused variable', ref: 'unused', path: 'web/app.ts', line: 7, source: 'eslint' },
  { id: 'GHSA-lodash', severity: 'critical', title: 'Prototype pollution', ref: 'GHSA-lodash', ecosystem: 'npm', package: 'lodash', version: '4.17.20', path: 'web/package-lock.json', source: 'npm audit' },
  { id: 'vet-1', severity: 'high', title: 'printf argument', ref: 'printf', path: 'src/main.go', line: 12, source: 'go vet' },
  { id: 'GHSA-axios', severity: 'high', title: 'Server-side request forgery', ref: 'GHSA-axios', ecosystem: 'npm', package: 'axios', version: '1.6.0', source: 'npm audit' },
  { id: 'GO-net', severity: 'medium', title: 'Excessive memory growth', ref: 'GO-2024-0001', ecosystem: 'go', package: 'golang.org/x/net', version: '0.1.0', source: 'osv' },
  { id: 'secret-1', severity: 'info', title: 'a note about the repository', source: 'trivy' },
];
const index = indexFindings({ findings: FINDINGS }, model);
const visibleWith = (filters = {}) => computeVisibility(model, {
  hiddenLangs: new Set(), hiddenEcosystems: new Set(), path: '', ...filters,
}).visible;

describe('the findings list, what it holds', () => {
  // Verifies: REQ-HUNT-049
  it('lists every finding, worst first and then by name', () => {
    const rows = findingRows(index, visibleWith());
    assert.deepEqual(rows.map(r => r.finding.id), ['GHSA-lodash', 'GHSA-axios', 'vet-1', 'GO-net', 'lint-1', 'secret-1']);
    // Each row leads to the building its bug stands at, which is where findings.js put it.
    for (const row of rows) assert.equal(row.node, index.place(row.finding));
    assert.equal(rows[0].node.id, 'p:npm:lodash', 'a package advisory belongs to the package');
    assert.equal(rows.at(-1).node, model.root, 'a finding about nothing in particular belongs to the repository');
  });

  // Verifies: REQ-HUNT-049
  it('leaves out what the filters took off the map', () => {
    // Hiding TypeScript takes web/app.ts away, and with it the npm packages nothing
    // else imports - three findings in all.
    const noTypeScript = findingRows(index, visibleWith({ hiddenLangs: new Set(['TypeScript']) }));
    assert.deepEqual(noTypeScript.map(r => r.finding.id), ['vet-1', 'GO-net', 'secret-1']);
    // An island sunk takes its packages' advisories with it, and nothing else.
    const noNpm = findingRows(index, visibleWith({ hiddenEcosystems: new Set(['e:npm']) }));
    assert.deepEqual(noNpm.map(r => r.finding.id), ['vet-1', 'GO-net', 'lint-1', 'secret-1']);
    // A path filter is a statement about files, so it takes a file's finding.
    const onlySource = findingRows(index, visibleWith({ path: 'src/**' }));
    assert.ok(!onlySource.some(r => r.finding.id === 'lint-1'), 'a finding on a filtered-out file is listed');
    assert.ok(onlySource.some(r => r.finding.id === 'vet-1'));
  });

  it('is empty without findings', () => {
    assert.deepEqual(findingRows(null, visibleWith()), []);
  });
});

describe('the findings list on the page', () => {
  let pack, list, element, pointed, opened;

  // The list wired to a backpack the way app.js wires it.
  beforeEach(() => {
    globalThis.document.activeElement = null;
    pack = new Backpack('findings-test');
    element = new El('ul');
    pointed = [];
    opened = [];
    list = new FindingList(element, {
      caught: id => pack.has(id),
      onPoint: row => pointed.push(row?.finding.id ?? null),
      onOpen: row => opened.push(row.finding.id),
      onToggle: ({ finding, node }, kept) => {
        if (kept) pack.remove(finding.id);
        else catchFinding(pack, index, finding, node);
        list.draw(findingRows(index, visibleWith()));
      },
    });
  });

  const rowFor = id => element.children.find(li => li.dataset.finding === id);

  // Verifies: REQ-HUNT-049
  it('shows the package or file, the severity, the id and title, and where it is', () => {
    const left = list.draw(findingRows(index, visibleWith()));
    assert.equal(left, 6, 'nothing is caught yet');
    const lodash = rowFor('GHSA-lodash');
    assert.equal(lodash.querySelector('.n').textContent, 'lodash@4.17.20');
    assert.equal(lodash.querySelector('.t').textContent, 'GHSA-lodash · Prototype pollution');
    assert.equal(lodash.querySelector('.w').textContent, 'npm · web/package-lock.json');
    assert.equal(lodash.querySelector('.state').textContent, 'critical');
    assert.ok(lodash.classList.contains('sev-critical'), 'the row does not carry its severity color');
    const vet = rowFor('vet-1');
    assert.equal(vet.querySelector('.n').textContent, 'src/main.go');
    assert.equal(vet.querySelector('.w').textContent, 'line 12');
    assert.equal(rowFor('secret-1').querySelector('.n').textContent, 'this repository');
  });

  // Verifies: REQ-HUNT-049
  it('lights a row\'s building while it is pointed at or focused, and opens it when picked', () => {
    list.draw(findingRows(index, visibleWith()));
    const vet = rowFor('vet-1');
    fire(vet, 'mouseenter');
    fire(vet, 'mouseleave');
    fire(vet, 'focus');
    fire(vet, 'blur');
    assert.deepEqual(pointed, ['vet-1', null, 'vet-1', null]);
    fire(vet, 'click');
    assert.deepEqual(opened, ['vet-1']);
  });

  // Verifies: REQ-HUNT-050
  it('puts a finding in the backpack from its button, once, and takes it out again', () => {
    list.draw(findingRows(index, visibleWith()));
    const button = rowFor('GO-net').querySelector('.keep');
    assert.equal(button.getAttribute('aria-pressed'), 'false');
    fire(button, 'click');
    assert.ok(pack.has('GO-net'));
    assert.deepEqual(opened, [], 'the button opened the row as well');
    // Drawn again from the backpack: marked as in it, and the count one fewer.
    const kept = rowFor('GO-net');
    assert.ok(kept.classList.contains('kept'));
    assert.equal(kept.querySelector('.keep').getAttribute('aria-pressed'), 'true');
    assert.equal(kept.querySelector('.state').textContent, 'in the backpack');
    assert.equal(list.draw(findingRows(index, visibleWith())), 5);
    // Catching it again from anywhere changes nothing: one entry, not two.
    assert.equal(catchFinding(pack, index, FINDINGS[4]), false);
    assert.equal(pack.items.length, 1);
    // Its button now takes it out, as the backpack's own does.
    fire(rowFor('GO-net').querySelector('.keep'), 'click');
    assert.ok(!pack.has('GO-net'));
    assert.equal(rowFor('GO-net').querySelector('.keep').getAttribute('aria-pressed'), 'false');
  });

  // Verifies: REQ-HUNT-051
  it('is worked through with the keys', () => {
    list.draw(findingRows(index, visibleWith()));
    const [first, second] = element.children;
    first.focus();
    assert.ok(fire(first, 'keydown', { key: 'ArrowDown' }).stopped, 'the arrow went on to the map as well');
    assert.equal(globalThis.document.activeElement, second);
    fire(second, 'keydown', { key: 'End' });
    assert.equal(globalThis.document.activeElement, element.children.at(-1));
    fire(element.children.at(-1), 'keydown', { key: 'Home' });
    assert.equal(globalThis.document.activeElement, first);
    fire(first, 'keydown', { key: 'Enter' });
    assert.deepEqual(opened, ['GHSA-lodash']);
    // `+` is the map's key for a level deeper, so the list keeps it to itself.
    assert.ok(fire(first, 'keydown', { key: '+' }).stopped);
    assert.ok(pack.has('GHSA-lodash'));
    // The redraw that followed kept the keyboard on the same finding.
    assert.equal(globalThis.document.activeElement, rowFor('GHSA-lodash'));
    fire(rowFor('GHSA-lodash'), 'keydown', { key: 'Insert' });
    assert.ok(!pack.has('GHSA-lodash'), 'Insert did not take it out again');
    // Keys the list does not use are left for the map.
    assert.equal(fire(rowFor('GHSA-lodash'), 'keydown', { key: 'b' }).stopped, false);
  });

  it('keeps the focus on a button that was pressed, across the redraw', () => {
    list.draw(findingRows(index, visibleWith()));
    const button = rowFor('vet-1').querySelector('.keep');
    button.focus();
    fire(button, 'click');
    assert.equal(globalThis.document.activeElement, rowFor('vet-1').querySelector('.keep'));
  });
});

describe('a finding caught from the list', () => {
  /** The bugs out on a map of the model's buildings, caught into `pack` when netted. */
  function street(pack) {
    const land = box('land', 0, 0, 40, 40, { y: -0.45, h: 0.45 });
    const buildings = ['f:src/main.go', 'f:web/app.ts', 'p:npm:lodash', 'p:npm:axios', 'p:go:golang.org/x/net', 'd:.']
      .map((id, i) => box('building', 3 * i, 0, 1, 1, { y: 0.2, h: 4, node: model.byId.get(id) }));
    const bugs = new Bugs(scene(), { onCatch: (f, node) => catchFinding(pack, index, f, node) });
    bugs.place(index, [land, ...buildings]);
    return bugs;
  }

  // Verifies: REQ-HUNT-050, REQ-HUNT-016
  it('is the same entry as its bug netted in the street', () => {
    const walked = new Backpack('walked'), listed = new Backpack('listed');
    const bugs = street(walked);
    const bug = bugs.bugs.find(b => b.f.id === 'vet-1');
    assert.ok(bug, 'the finding has no bug to catch');
    assert.ok(bugs.catch(bug, 'net'));

    const row = findingRows(index, visibleWith()).find(r => r.finding.id === 'vet-1');
    assert.ok(catchFinding(listed, index, row.finding, row.node));
    const strip = ({ caughtAt, ...rest }) => rest; // when, not what
    assert.deepEqual(listed.items.map(strip), walked.items.map(strip));
    assert.equal(listed.items[0].nodeId, 'f:src/main.go');
    // And the backpack changed in the same way, so whatever follows a change - the
    // server hearing of it (app.js drawPack) - follows from both.
    assert.equal(walked.counts.total, listed.counts.total);
  });

  // Verifies: REQ-HUNT-050, REQ-HUNT-029
  it('takes its bug off the street without anybody walking there', () => {
    const pack = new Backpack('street');
    const bugs = street(pack);
    const bug = bugs.bugs.find(b => b.f.id === 'GHSA-lodash');
    const before = bugs.counts;
    assert.equal(bugs.at(bug.position), bug, 'the bug cannot be aimed at to begin with');

    const row = findingRows(index, visibleWith()).find(r => r.finding.id === 'GHSA-lodash');
    catchFinding(pack, index, row.finding, row.node);
    // What the backpack's change does on the map (app.js drawPack).
    bugs.keepCaught(pack.ids);
    assert.equal(bug.caught, true);
    assert.equal(bugs.counts.caught, before.caught + 1, 'the tally did not count it');
    assert.equal(bugs.at(bug.position), null, 'a caught bug can still be aimed at');
    // Put back from the backpack, it walks again.
    pack.remove('GHSA-lodash');
    bugs.keepCaught(pack.ids);
    assert.equal(bugs.at(bug.position), bug);
  });
});
