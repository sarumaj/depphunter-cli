// The Findings view's catch against the map's own.
//
// The page owns the backpack, and a catch there records a finding against the node
// its bug stands at (web/static/findings.js) in the shape web/static/backpack.js
// writes. The extension cannot run those modules, so it says the same in TypeScript
// (findings.ts) - and a copy is only worth having while it agrees. So both are run
// here on one graph and one findings document, and the entries compared: whatever
// the Findings view puts in the backpack is what catching the bug on the map would
// have put there.

const assert = require('node:assert');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { before, describe, it } = require('node:test');

require('./stub');
const { FindingsView, packItemFor, placeFinding } = require('../out/findings.js');

const STATIC = path.join(__dirname, '..', '..', 'web', 'static');
const load = name => import(pathToFileURL(path.join(STATIC, name)).href);

const directory = (p, parent) => ({ id: `d:${p}`, kind: 'dir', name: p.split('/').pop(), path: p, parent });
const file = (p, parent, language) => ({ id: `f:${p}`, kind: 'file', name: p.split('/').pop(), path: p, parent, lang: language, loc: 10 });

const GRAPH = {
  nodes: [
    { id: 'd:.', kind: 'dir', name: '.', path: '.' },
    directory('src', 'd:.'), directory('src/deep', 'd:src'),
    file('src/main.go', 'd:src', 'Go'), file('src/deep/x.go', 'd:src/deep', 'Go'),
    { id: 'e:npm', kind: 'ecosystem', name: 'npm' },
    { id: 'p:npm:lodash', kind: 'package', name: 'lodash', parent: 'e:npm' },
  ],
  edges: [{ from: 'f:src/main.go', to: 'p:npm:lodash', kind: 'import' }],
};

// One of every way a finding is placed: on its package, on its file, on the nearest
// directory the map draws for a path it does not, and on the repository.
const FINDINGS = [
  { id: 'GHSA-1', severity: 'critical', title: 'Prototype pollution', ecosystem: 'npm', package: 'lodash', version: '4.17.20', path: 'package-lock.json' },
  { id: 'vet-1', severity: 'high', title: 'printf', path: 'src/main.go', line: 12 },
  { id: 'lock-1', severity: 'medium', title: 'in a lock file', path: 'src/deep/go.sum', line: 3 },
  { id: 'far-1', severity: 'low', title: 'nowhere drawn', path: 'vendor/a/b.go' },
  { id: 'pkg-gone', severity: 'medium', title: 'a package not on the map', ecosystem: 'npm', package: 'left-pad', path: 'src/main.go' },
  { id: 'repo-1', severity: '', title: '' },
];

describe('the Findings view against the map', () => {
  let model, index, Backpack;

  before(async () => {
    // backpack.js reads and writes the browser's store, and does without one.
    const { buildModel } = await load('model.js');
    const { indexFindings } = await load('findings.js');
    ({ Backpack } = await load('backpack.js'));
    model = buildModel(GRAPH);
    index = indexFindings({ findings: FINDINGS }, model);
  });

  // Verifies: REQ-EXT-036
  it('places a finding on the node the map places its bug on', () => {
    const has = id => model.byId.has(id);
    for (const f of FINDINGS) {
      assert.strictEqual(placeFinding(f, has), index.place(f).id, `${f.id} is placed elsewhere than on the map`);
    }
  });

  // Verifies: REQ-EXT-036
  it('makes the entry the map\'s catch makes', () => {
    const has = id => model.byId.has(id);
    for (const f of FINDINGS) {
      const pack = new Backpack(`cross-check-${f.id}`);
      assert.ok(pack.add(f, index.place(f)));
      const [page] = pack.items;
      const ours = packItemFor(f, placeFinding(f, has), page.caughtAt);
      assert.deepStrictEqual(ours, page, `${f.id} differs from the map's entry`);
      assert.deepStrictEqual(Object.keys(ours), Object.keys(page), 'the fields are in another order');
    }
  });

  // Verifies: REQ-EXT-035
  it('lists worst first and then by name, and says which are caught', () => {
    const view = new FindingsView(id => model.byId.has(id));
    let fired = 0;
    view.onDidChangeTreeData(() => fired++);
    view.setFindings(FINDINGS);
    assert.deepStrictEqual(view.getChildren().map(f => f.id), ['GHSA-1', 'vet-1', 'pkg-gone', 'lock-1', 'far-1', 'repo-1']);
    view.setCaught(['vet-1']);
    assert.strictEqual(view.getTreeItem(FINDINGS[1]).contextValue, 'caught');
    assert.strictEqual(view.getTreeItem(FINDINGS[0]).contextValue, 'uncaught');
    const before = fired;
    view.setCaught(['vet-1']); // the same backpack again is not a change
    assert.strictEqual(fired, before);
    const item = view.getTreeItem(FINDINGS[0]);
    assert.strictEqual(item.label, 'lodash@4.17.20');
    assert.strictEqual(item.iconPath.id, 'error');
    assert.strictEqual(item.command.arguments[0].nodeId, 'p:npm:lodash');
    assert.strictEqual(view.getTreeItem(FINDINGS[5]).label, 'this repository');
  });
});
