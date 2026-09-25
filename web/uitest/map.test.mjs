// The archipelago: what the layout draws for a repository, and what a selection
// draws on top of it.
//
// Everything the map shows is a list of boxes (layout.js) plus, for a selection, a
// list of arcs between them (model.js focusArcs). Both are plain functions of the
// graph document and the view state, so the map's promises - containment, heights,
// islands in rings, the same map for the same tree, arcs only for the selection - are
// checked here on those lists rather than on pixels.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { buildModel, focusArcs, expandToLevel, toggles, isWithin } = await import('../static/model.js');
const { computeVisibility } = await import('../static/filter.js');
const { layout, representative, SCALES } = await import('../static/layout.js');
const { boxColor, languageColors, assignSlots } = await import('../static/colors.js');
const { effectiveMode, computeMetrics, historyT } = await import('../static/history.js');
const { MapScene } = await import('../static/scene.js');
const THREE = await import('../static/vendor/three.module.min.js');

// ---------------------------------------------------------------- fixtures

const ROOT = { id: 'd:.', kind: 'dir', name: '.', path: '.' };
const dir = (path, parent = 'd:.') => ({ id: `d:${path}`, kind: 'dir', name: path.split('/').pop(), path, parent });
const file = (path, parent = 'd:.', extra = {}) =>
  ({ id: `f:${path}`, kind: 'file', name: path.split('/').pop(), path, parent, lang: 'go', ...extra });
const sym = (file, name, symbolKind) =>
  ({ id: `s:${file}#${name}`, kind: 'symbol', name, symbolKind, parent: `f:${file}` });
const eco = name => ({ id: `e:${name}`, kind: 'ecosystem', name });
const pkg = (ecosystem, name) => ({ id: `p:${ecosystem}:${name}`, kind: 'package', name, parent: `e:${ecosystem}` });
const imp = (from, to, kind = 'import') => ({ from, to, kind });

/**
 * A small repository with something of everything: a nested directory, a file with
 * symbols, one without, an empty file, two ecosystems and edges both inside src/ and
 * across its boundary in each direction.
 */
function repo() {
  return {
    nodes: [
      ROOT,
      dir('src'), dir('src/util', 'd:src'), dir('lib'),
      file('main.go', 'd:.', { loc: 1000 }),
      file('empty.go', 'd:.', { loc: 0 }),
      file('src/a.go', 'd:src', { loc: 300 }),
      file('src/b.go', 'd:src', { loc: 120 }),
      file('src/util/u.go', 'd:src/util', { loc: 50 }),
      file('lib/l1.go', 'd:lib', { loc: 80 }),
      file('lib/l2.go', 'd:lib', { loc: 90 }),
      sym('lib/l1.go', 'Store', 'type'), sym('lib/l1.go', 'Open', 'func'),
      sym('lib/l1.go', 'Close', 'method'), sym('lib/l1.go', 'limit', 'var'),
      eco('npm'), pkg('npm', 'react'), pkg('npm', 'lodash'),
      eco('pypi'), pkg('pypi', 'requests'),
    ],
    edges: [
      imp('f:main.go', 'f:src/a.go'),
      imp('f:src/a.go', 'f:lib/l1.go'),
      imp('f:lib/l1.go', 'f:src/b.go'),
      imp('f:src/a.go', 'f:src/b.go'), // inside src/
      imp('f:src/a.go', 'p:npm:react'),
      imp('f:src/b.go', 'p:npm:react'),
      imp('f:src/util/u.go', 'p:npm:react'),
      imp('f:main.go', 'p:npm:lodash'),
      imp('f:main.go', 'p:pypi:requests'),
    ],
  };
}

const noFilters = { hiddenLangs: new Set(), hiddenEcosystems: new Set(), path: '' };

/** Lays a graph out the way the app does: the root always open, `expanded` besides. */
function draw(graph, { expanded = [], heightScale = 'sqrt' } = {}) {
  const model = buildModel(graph);
  const vis = computeVisibility(model, noFilters);
  const L = layout(model, { vis, expanded: new Set(['d:.', ...expanded]), heightScale });
  return { model, vis, L };
}

const EPS = 1e-9;
const edges = b => ({ x0: b.x - b.w / 2, x1: b.x + b.w / 2, z0: b.z - b.d / 2, z1: b.z + b.d / 2 });
/** Whether `inner`'s footprint lies within `outer`'s. */
const within = (inner, outer) => {
  const a = edges(inner), b = edges(outer);
  return a.x0 >= b.x0 - EPS && a.x1 <= b.x1 + EPS && a.z0 >= b.z0 - EPS && a.z1 <= b.z1 + EPS;
};
/** Whether two footprints share any area (touching edges do not count). */
const overlap = (p, q) => {
  const a = edges(p), b = edges(q);
  return a.x0 < b.x1 - EPS && b.x0 < a.x1 - EPS && a.z0 < b.z1 - EPS && b.z0 < a.z1 - EPS;
};
const near = (a, b, msg) => assert.ok(Math.abs(a - b) < 1e-9, `${msg}: ${a} != ${b}`);

/** The land box an ecosystem or the root stands on. */
const landOf = (L, id) => L.boxes.find(b => b.kind === 'land' && b.node.id === id);

/** What focusArcs needs from the app, for a layout. */
const focusOpts = (L, vis, extra = {}) => ({
  rep: n => representative(L.byNode, n), visible: vis.visible, outColor: 'out', inColor: 'in', ...extra,
});

// ---------------------------------------------------------------- the land

describe('the mainland', () => {
  // Verifies: REQ-MAP-001
  it('is one land box under the root that everything in the repository stands on', () => {
    const { model, L } = draw(repo(), { expanded: ['d:src', 'd:src/util', 'd:lib'] });
    const lands = L.boxes.filter(b => b.kind === 'land' && b.node === model.root);
    assert.equal(lands.length, 1);
    const [main] = lands;
    const terrace = L.byNode.get('d:.');
    assert.ok(main.w > terrace.w && main.d > terrace.d, 'the land does not extend beyond the root terrace');
    near(main.w - terrace.w, main.d - terrace.d, 'the margin differs between the sides');
    for (const b of L.boxes) {
      if (b.kind === 'land' || !isWithin(b.node, model.root)) continue;
      assert.ok(within(b, main), `${b.kind} ${b.node.id} lies outside the mainland`);
    }
    // ... and nothing from outside stands on it.
    const packages = L.boxes.filter(b => b.kind === 'package');
    assert.ok(packages.length > 0);
    for (const p of packages) assert.ok(!overlap(p, main), `${p.node.id} stands on the mainland`);
  });
});

describe('directories', () => {
  // Verifies: REQ-MAP-002
  it('open into a terrace on their parent terrace, carrying their children on top', () => {
    const { model, L } = draw(repo(), { expanded: ['d:src'] });
    const src = L.byNode.get('d:src');
    const root = L.byNode.get('d:.');
    assert.equal(src.kind, 'terrace');
    near(src.y, root.y + root.h, 'the terrace does not stand on its parent');
    const children = model.byId.get('d:src').children;
    assert.equal(children.length, 3);
    for (const c of children) {
      const b = L.byNode.get(c.id);
      assert.ok(within(b, src), `${c.id} is off its terrace`);
      near(b.y, src.y + src.h, `${c.id} is not on top of the terrace`);
    }
  });

  // Verifies: REQ-MAP-003
  it('fold into one district whose side grows with the root of the file count', () => {
    const files = (d, n) => Array.from({ length: n }, (_, i) => file(`${d}/f${i}.go`, `d:${d}`, { loc: 10 }));
    const { L } = draw({
      nodes: [ROOT, dir('one'), dir('four'), dir('sixteen'), ...files('one', 1), ...files('four', 4), ...files('sixteen', 16)],
      edges: [],
    });
    const [one, four, sixteen] = ['d:one', 'd:four', 'd:sixteen'].map(id => L.byNode.get(id));
    for (const b of [one, four, sixteen]) {
      assert.equal(b.kind, 'district');
      assert.equal(L.boxes.filter(x => x.node === b.node).length, 1, 'a collapsed directory is more than one box');
      near(b.w, b.d, 'a district is not square');
    }
    near(one.w, 1.4, 'the smallest district is not at the minimum side');
    assert.ok(four.w > 1.4);
    near(sixteen.w / four.w, 2, 'four times the files is not twice the side');
  });
});

describe('files', () => {
  // Verifies: REQ-MAP-005
  it('stand as 1 x 1 buildings from the floor to ten units for the longest', () => {
    for (const heightScale of Object.keys(SCALES)) {
      const { L } = draw(repo(), { expanded: ['d:src', 'd:src/util', 'd:lib'], heightScale });
      const buildings = L.boxes.filter(b => b.node.kind === 'file');
      assert.equal(buildings.length, 7);
      for (const b of buildings) {
        assert.equal(b.kind, 'building');
        assert.equal(b.w, 1);
        assert.equal(b.d, 1);
      }
      near(L.byNode.get('f:main.go').h, 10.2, `${heightScale}: the longest file`);
      near(L.byNode.get('f:empty.go').h, 0.2, `${heightScale}: the empty file`);
    }
  });

  // Verifies: REQ-MAP-006
  it('open into a plateau with one block per symbol, as tall as its kind', () => {
    const { model, L } = draw(repo(), { expanded: ['d:lib', 'f:lib/l1.go'] });
    const plateau = L.byNode.get('f:lib/l1.go');
    assert.equal(plateau.kind, 'terrace');
    assert.ok(!L.boxes.some(b => b.node.id === 'f:lib/l1.go' && b.kind === 'building'), 'the building is still there');
    const blocks = L.boxes.filter(b => b.kind === 'symbol');
    assert.equal(blocks.length, model.byId.get('f:lib/l1.go').children.length);
    const height = Object.fromEntries(blocks.map(b => [b.node.name, b.h]));
    assert.deepEqual(height, { Store: 1.1, Open: 0.7, Close: 0.7, limit: 0.35 });
    for (const b of blocks) {
      assert.ok(within(b, plateau), `${b.node.name} is off its plateau`);
      near(b.y, plateau.y + plateau.h, `${b.node.name} is not on the plateau`);
    }
    for (let i = 0; i < blocks.length; i++) {
      for (let j = i + 1; j < blocks.length; j++) assert.ok(!overlap(blocks[i], blocks[j]));
    }
  });

  // Verifies: REQ-MAP-006, REQ-MAP-022
  it('without symbols have nothing to open and stay buildings', () => {
    const { model, L } = draw(repo(), { expanded: ['d:lib', 'f:lib/l2.go'] });
    assert.equal(toggles(model.byId.get('f:lib/l2.go'), new Set(['f:lib/l2.go'])), '');
    assert.equal(L.byNode.get('f:lib/l2.go').kind, 'building');
  });

  // Verifies: REQ-MAP-061
  it('keep their heights when a binary larger than all of them arrives', () => {
    // The blob has no counted lines, only 4 MB of bytes - a hundred thousand lines'
    // worth by the bytes-per-line estimate, a hundred times the longest file.
    const before = draw(repo(), { expanded: ['d:src', 'd:lib'] }).L;
    const g = repo();
    g.nodes.push(file('model.glb', 'd:.', { lang: '', bytes: 4_000_000 }));
    const after = draw(g, { expanded: ['d:src', 'd:lib'] }).L;
    for (const b of before.boxes) {
      if (b.kind !== 'building') continue;
      near(after.byNode.get(b.node.id).h, b.h, `${b.node.id} changed height`);
    }
    const blob = after.byNode.get('f:model.glb').h;
    assert.ok(blob <= after.byNode.get('f:main.go').h + EPS, `the binary (${blob}) is taller than the longest file`);
  });
});

describe('height scales', () => {
  // Verifies: REQ-MAP-038
  it('are linear, square root and logarithmic, and a relayout follows the choice', () => {
    near(SCALES.log(0.5), Math.log(1 + 500) / Math.log(1001), 'log');
    near(SCALES.log(1), 1, 'log of the largest');
    near(SCALES.sqrt(0.25), 0.5, 'sqrt');
    near(SCALES.linear(0.3), 0.3, 'linear');

    // Linear: above the floor, height is in proportion to lines.
    const lin = draw(repo(), { expanded: ['d:src'], heightScale: 'linear' }).L;
    const above = id => lin.byNode.get(id).h - 0.2;
    near(above('f:src/a.go') / above('f:src/b.go'), 300 / 120, 'linear is not proportional');

    // The same file comes out differently on each, the compressed scales raising it.
    const h = s => draw(repo(), { expanded: ['d:src'], heightScale: s }).L.byNode.get('f:src/b.go').h;
    assert.ok(h('linear') < h('sqrt') && h('sqrt') < h('log'), `${h('linear')} ${h('sqrt')} ${h('log')}`);
    // An unknown scale is the default one.
    near(h('nonsense'), h('sqrt'), 'the default scale');
  });
});

// ---------------------------------------------------------------- islands

describe('islands', () => {
  // Verifies: REQ-MAP-007
  it('are one per ecosystem, off the mainland, each carrying its own packages', () => {
    const { model, L } = draw(repo());
    const main = landOf(L, 'd:.');
    const npm = landOf(L, 'e:npm'), pypi = landOf(L, 'e:pypi');
    assert.ok(npm && pypi, 'an ecosystem has no island');
    assert.equal(L.boxes.filter(b => b.kind === 'land').length, 3);
    for (const island of [npm, pypi]) assert.ok(!overlap(island, main), `${island.node.id} overlaps the mainland`);
    assert.ok(!overlap(npm, pypi));
    for (const b of L.boxes.filter(x => x.kind === 'package')) {
      const own = landOf(L, b.node.parentNode.id);
      assert.ok(within(b, own), `${b.node.id} is off its island`);
      for (const e of model.ecosystems) {
        if (e !== b.node.parentNode) assert.ok(!overlap(b, landOf(L, e.id)), `${b.node.id} is on ${e.id}`);
      }
    }
  });

  // Verifies: REQ-MAP-008
  it('carry packages as tall as the files importing them, depends edges aside', () => {
    const { L } = draw(repo());
    const react = L.byNode.get('p:npm:react').h; // three importers
    const requests = L.byNode.get('p:pypi:requests').h; // one
    assert.ok(react > requests, `react ${react} is not taller than requests ${requests}`);
    near(react, 5.3, 'the most imported package');

    // A package depending on another is not a file importing it.
    const g = repo();
    g.edges.push(imp('p:npm:react', 'p:pypi:requests', 'depends'), imp('p:npm:lodash', 'p:pypi:requests', 'depends'));
    near(draw(g).L.byNode.get('p:pypi:requests').h, requests, 'a depends edge raised the package');
  });

  /**
   * A mainland of one file and `uses.length` ecosystems of one package each, the i-th
   * imported uses[i] times - equal islands that differ only in how much they are used.
   */
  function ringed(uses) {
    const nodes = [ROOT], edgesOut = [];
    uses.forEach((n, i) => {
      const name = `eco${i}`;
      nodes.push(eco(name), pkg(name, 'p'));
      for (let k = 0; k < n; k++) {
        nodes.push(file(`${name}-${k}.go`, 'd:.', { loc: 10 }));
        edgesOut.push(imp(`f:${name}-${k}.go`, `p:${name}:p`));
      }
    });
    return draw({ nodes, edges: edgesOut }).L;
  }

  /** How far an island's land is from the mainland's, across the water. */
  const gap = (L, id) => {
    const a = edges(landOf(L, 'd:.')), b = edges(landOf(L, id));
    return Math.max(b.x0 - a.x1, a.x0 - b.x1, b.z0 - a.z1, a.z0 - b.z1);
  };
  /** Which side of the mainland an island is on. */
  const side = (L, id) => {
    const a = edges(landOf(L, 'd:.')), b = edges(landOf(L, id));
    if (b.z1 <= a.z0 + EPS) return 'n';
    if (b.z0 >= a.z1 - EPS) return 's';
    if (b.x0 >= a.x1 - EPS) return 'e';
    if (b.x1 <= a.x0 + EPS) return 'w';
    return '?';
  };
  const islandsOf = L => L.boxes.filter(b => b.kind === 'land' && b.node.kind === 'ecosystem');

  // Verifies: REQ-MAP-011
  it('ring the mainland, one to a side when four are equal', () => {
    // The mainland here is exactly as wide as one island: a side's row holds one.
    const L = ringed([1, 1, 1, 1]);
    const sides = islandsOf(L).map(b => side(L, b.node.id)).sort();
    assert.deepEqual(sides, ['e', 'n', 's', 'w']);
  });

  // Verifies: REQ-MAP-011
  it('start a further ring when no side has room, most imported nearest', () => {
    const uses = [1, 2, 3, 4, 5, 6, 7, 8];
    const L = ringed(uses);
    const islands = islandsOf(L);
    assert.equal(islands.length, 8);
    for (let i = 0; i < islands.length; i++) {
      assert.ok(!overlap(islands[i], landOf(L, 'd:.')), `${islands[i].node.id} is on the mainland`);
      for (let j = i + 1; j < islands.length; j++) {
        assert.ok(!overlap(islands[i], islands[j]), `${islands[i].node.id} overlaps ${islands[j].node.id}`);
      }
    }
    const inner = Math.min(...islands.map(b => gap(L, b.node.id)));
    const ring = id => (gap(L, id) - inner < 1e-6 ? 1 : 2);
    // The four most imported take the first ring, the rest go round outside it.
    for (let i = 0; i < uses.length; i++) {
      assert.equal(ring(`e:eco${i}`), uses[i] > 4 ? 1 : 2, `eco${i} (imported ${uses[i]} times)`);
    }
  });
});

// ---------------------------------------------------------------- stability

describe('positions', () => {
  const where = L => L.boxes.map(b => [b.node.id, b.kind, b.x, b.z, b.y, b.w, b.d]);

  // Verifies: REQ-MAP-010, REQ-MAP-043
  it('are the same for the same tree, whatever order it arrives in', () => {
    const opts = { expanded: ['d:src', 'd:src/util', 'd:lib', 'f:lib/l1.go'] };
    const once = draw(repo(), opts).L;
    assert.deepEqual(where(draw(repo(), opts).L), where(once));
    // The graph document lists nodes in whatever order the analysis found them; the
    // layout orders children itself.
    const g = repo();
    g.nodes.reverse();
    g.edges.reverse();
    const shuffled = draw(g, opts).L;
    for (const b of once.boxes) {
      if (b.kind === 'land') continue;
      const s = shuffled.byNode.get(b.node.id);
      assert.deepEqual([s.x, s.z, s.y, s.w, s.d], [b.x, b.z, b.y, b.w, b.d], `${b.node.id} moved`);
    }
  });

  // Verifies: REQ-MAP-010
  it('do not follow the edges', () => {
    // An import between two files of the repository is not part of the hierarchy, so
    // nothing on the map may move for it.
    const opts = { expanded: ['d:src', 'd:lib'] };
    const before = draw(repo(), opts).L;
    const g = repo();
    g.edges.push(imp('f:lib/l2.go', 'f:main.go'), imp('f:empty.go', 'f:src/util/u.go'));
    assert.deepEqual(where(draw(g, opts).L), where(before));
  });

  // Verifies: REQ-MAP-043
  it('pack every terrace without two children overlapping', () => {
    // Directories of mixed sizes, so that the packing has districts of several sides
    // and buildings to fit together, and nested terraces to fit inside others.
    const nodes = [ROOT];
    const sizes = [1, 3, 9, 2, 25, 5, 14, 7];
    sizes.forEach((n, i) => {
      nodes.push(dir(`d${i}`));
      for (let k = 0; k < n; k++) nodes.push(file(`d${i}/f${k}.go`, `d:d${i}`, { loc: 10 * (k + 1) }));
      nodes.push(dir(`d${i}/sub`, `d:d${i}`), file(`d${i}/sub/x.go`, `d:d${i}/sub`, { loc: 5 }));
    });
    for (let k = 0; k < 6; k++) nodes.push(file(`top${k}.go`, 'd:.', { loc: 40 }));
    const expanded = sizes.flatMap((_, i) => (i % 2 ? [`d:d${i}`] : []));
    const { L } = draw({ nodes, edges: [] }, { expanded });
    const terraces = L.boxes.filter(b => b.kind === 'terrace');
    assert.ok(terraces.length >= 5);
    for (const t of terraces) {
      const kids = L.boxes.filter(b => b.kind !== 'land' && b.node.parentNode === t.node);
      assert.ok(kids.length > 0);
      for (let i = 0; i < kids.length; i++) {
        assert.ok(within(kids[i], t), `${kids[i].node.id} is off ${t.node.id}`);
        for (let j = i + 1; j < kids.length; j++) {
          assert.ok(!overlap(kids[i], kids[j]), `${kids[i].node.id} overlaps ${kids[j].node.id}`);
        }
      }
    }
  });
});

// ---------------------------------------------------------------- expanding

describe('expanding and collapsing', () => {
  // Verifies: REQ-MAP-022
  it('opens a district into its terrace and shuts it again', () => {
    const { model } = draw(repo());
    const src = model.byId.get('d:src');
    const expanded = new Set(['d:.']);
    const vis = computeVisibility(model, noFilters);
    const at = () => layout(model, { vis, expanded, heightScale: 'sqrt' });

    assert.equal(toggles(src, expanded), 'open');
    assert.equal(at().byNode.get('d:src').kind, 'district');
    expanded.add(src.id);
    assert.equal(toggles(src, expanded), 'close');
    const open = at();
    assert.equal(open.byNode.get('d:src').kind, 'terrace');
    assert.equal(open.byNode.get('f:src/a.go').kind, 'building');
    expanded.delete(src.id);
    const shut = at();
    assert.equal(shut.byNode.get('d:src').kind, 'district');
    assert.equal(shut.byNode.get('f:src/a.go'), undefined, 'a file of a shut directory is still drawn');
  });

  // Verifies: REQ-MAP-022
  it('opens a file into its symbols, and a symbol toggles its file', () => {
    const { model } = draw(repo());
    const l1 = model.byId.get('f:lib/l1.go');
    const expanded = new Set(['d:.', 'd:lib']);
    assert.equal(toggles(l1, expanded), 'open');
    assert.equal(toggles(l1.children[0], expanded), 'open', 'a symbol does not stand for its file');
    expanded.add(l1.id);
    assert.equal(toggles(l1.children[0], expanded), 'close');
    const L = layout(model, { vis: computeVisibility(model, noFilters), expanded, heightScale: 'sqrt' });
    assert.equal(L.boxes.filter(b => b.kind === 'symbol').length, 4);
    // Packages and islands are not things that open.
    assert.equal(toggles(model.byId.get('p:npm:react'), expanded), '');
  });

  // Verifies: REQ-MAP-023
  it('steps every directory to a depth, between 1 and the deepest level', () => {
    const { model } = draw(repo());
    const vis = computeVisibility(model, noFilters);
    // Depths: the root 0, src/ and lib/ 1, src/util/ 2 - three levels.
    const two = expandToLevel(model, 2);
    assert.equal(two.level, 2);
    assert.equal(two.maxDepth, 3);
    assert.deepEqual([...two.expanded].sort(), ['d:.', 'd:lib', 'd:src']);
    assert.equal(layout(model, { vis, expanded: two.expanded }).byNode.get('d:src/util').kind, 'district');

    // + from 2: every directory at depth 2 opens as well.
    const three = expandToLevel(model, two.level + 1);
    assert.equal(three.level, 3);
    assert.ok(three.expanded.has('d:src/util'));
    assert.equal(layout(model, { vis, expanded: three.expanded }).byNode.get('d:src/util').kind, 'terrace');

    assert.equal(expandToLevel(model, three.level + 1).level, 3, 'went past the deepest level');
    assert.equal(expandToLevel(model, 0).level, 1, 'went below 1');
    assert.deepEqual([...expandToLevel(model, 0).expanded], ['d:.']);
  });
});

// ---------------------------------------------------------------- arcs

describe('arcs', () => {
  // Verifies: REQ-MAP-012
  it('are drawn only for a selection, each with the selection at one end', () => {
    const { model, vis, L } = draw(repo(), { expanded: ['d:src', 'd:lib'] });
    const opts = focusOpts(L, vis);
    assert.equal(focusArcs(model, null, opts), null, 'arcs with nothing selected');

    const main = model.byId.get('f:main.go');
    const focus = focusArcs(model, main, opts);
    assert.equal(focus.selBox, L.byNode.get('f:main.go'));
    // main.go imports src/a.go, lodash and requests; nothing imports it.
    assert.deepEqual(focus.arcs.map(a => a.to.node.id).sort(), ['f:src/a.go', 'p:npm:lodash', 'p:pypi:requests']);
    for (const a of focus.arcs) assert.ok(a.from === focus.selBox || a.to === focus.selBox);

    // Clearing the selection is selecting nothing: no arcs again.
    assert.equal(focusArcs(model, null, opts), null);
  });

  // Verifies: REQ-MAP-012
  it('are capped, the largest counts kept', () => {
    const { model, vis, L } = draw(repo());
    const src = model.byId.get('d:src');
    const [top] = focusArcs(model, src, focusOpts(L, vis, { max: 1 })).arcs;
    assert.equal(top.to.node.id, 'p:npm:react');
    assert.equal(top.count, 3);
  });

  // Verifies: REQ-MAP-009
  it('join the representatives of both ends, and nothing to itself', () => {
    const { model, vis, L } = draw(repo()); // src/ and lib/ collapsed
    const rep = n => representative(L.byNode, n);
    assert.equal(rep(model.byId.get('f:src/util/u.go')), L.byNode.get('d:src'));
    assert.equal(rep(model.byId.get('f:main.go')), L.byNode.get('f:main.go'));

    // main.go imports a file folded into src/: the arc lands on the district.
    const main = focusArcs(model, model.byId.get('f:main.go'), focusOpts(L, vis));
    assert.ok(main.arcs.some(a => a.to === L.byNode.get('d:src') && a.to.kind === 'district'));

    // Selecting a.go inside the shut src/: its edge to b.go joins the district to
    // itself and is not drawn, where its edge to lib/l1.go is.
    const a = focusArcs(model, model.byId.get('f:src/a.go'), focusOpts(L, vis));
    assert.equal(a.selBox, L.byNode.get('d:src'));
    assert.ok(a.arcs.every(x => x.from !== x.to), 'an arc from a box to itself');
    assert.ok(a.arcs.some(x => x.to === L.byNode.get('d:lib')));
  });

  // Verifies: REQ-MAP-009
  it('end in an arrow head at the target', () => {
    const { model, vis, L } = draw(repo(), { expanded: ['d:src'] });
    const { arcs } = focusArcs(model, model.byId.get('f:main.go'), focusOpts(L, vis, { outColor: '#ff0000' }));
    // setArcs only needs somewhere to put the meshes; the rest of MapScene is WebGL.
    const fake = { edgeGroup: new THREE.Group(), bendable: m => m, requestRender() {} };
    MapScene.prototype.setArcs.call(fake, arcs);
    const meshes = fake.edgeGroup.children;
    assert.equal(meshes.length, 2 * arcs.length, 'an arc is not a tube and a head');
    arcs.forEach((a, i) => {
      const head = meshes[2 * i + 1];
      assert.equal(head.geometry.type, 'ConeGeometry');
      const d = (b, p) => Math.hypot(p.x - b.x, p.z - b.z);
      assert.ok(d(a.to, head.position) < d(a.from, head.position), `the head of arc ${i} is at its source`);
    });
  });

  // Verifies: REQ-MAP-013
  it('from a directory aggregate its files, one per pair and direction with a count', () => {
    const { model, vis, L } = draw(repo()); // src/ collapsed
    const src = model.byId.get('d:src');
    const { arcs, selBox } = focusArcs(model, src, focusOpts(L, vis));
    const key = a => `${a.from.node.id}>${a.to.node.id}:${a.color}`;
    const counts = Object.fromEntries(arcs.map(a => [key(a), a.count]));
    assert.deepEqual(counts, {
      // Three files in three places under src/ import react: one arc, counted 3.
      'd:src>p:npm:react:out': 3,
      // src/ to lib/ and lib/ to src/ are two arcs, not one.
      'd:src>d:lib:out': 1,
      'd:lib>d:src:in': 1,
      'f:main.go>d:src:in': 1,
    });
    // a.go -> b.go stays inside src/.
    assert.ok(arcs.every(a => a.from === selBox || a.to === selBox));
    assert.ok(!arcs.some(a => a.from === a.to));

    // Opened, the directory is its terrace and its edges are still aggregated.
    const open = draw(repo(), { expanded: ['d:src', 'd:src/util'] });
    const again = focusArcs(open.model, open.model.byId.get('d:src'), focusOpts(open.L, open.vis));
    assert.equal(again.selBox.kind, 'terrace');
    assert.equal(again.arcs.find(a => a.to.node.id === 'p:npm:react').count, 3);
  });
});

// ---------------------------------------------------------------- color

describe('coloring', () => {
  // A palette whose every role is its own name, so a color says where it came from.
  const seq = ['s1', 's2', 's3', 's4', 's5', 's6', 's7'];
  const pal = {
    series: ['go', 'js', 'py', 'c4', 'c5', 'c6', 'c7'], other: 'other', seq, noData: 'none',
    land: 'land', terraceA: 'ta', terraceB: 'tb', district: 'district',
    pkg: 'pkg', pkgUnresolved: 'pkg?', pkgFloating: 'pkg~',
  };
  const g = repo();
  g.nodes.push(file('web/app.js', 'd:.', { lang: 'js', loc: 200 }));
  const { model, L } = draw(g, { expanded: ['d:src', 'd:lib', 'f:lib/l1.go'] });
  const langs = languageColors(model, pal, assignSlots(model));
  const sizeT = loc => Math.sqrt(Math.min(1, loc / 1000));
  const color = (id, mode, extra = {}) =>
    boxColor(L.byNode.get(id), { mode, pal, langs, sizeT, hm: false, historyT, ...extra });

  // Verifies: REQ-MAP-037
  it('by language gives each language its own color', () => {
    const go = color('f:main.go', 'language'), js = color('f:web/app.js', 'language');
    assert.ok(pal.series.includes(go) && pal.series.includes(js));
    assert.notEqual(go, js);
    assert.equal(color('f:src/a.go', 'language'), go);
    // A symbol takes its file's.
    assert.equal(color('s:lib/l1.go#Store', 'language'), go);
  });

  // Verifies: REQ-MAP-037
  it('by size walks the sequential ramp with the drawn size', () => {
    const at = id => seq.indexOf(color(id, 'size'));
    for (const id of ['f:main.go', 'f:src/a.go', 'f:empty.go', 'd:src/util', 's:lib/l1.go#Open']) {
      assert.ok(at(id) >= 0, `${id} is off the ramp`);
    }
    assert.equal(at('f:main.go'), seq.length - 1);
    assert.equal(at('f:empty.go'), 0);
    assert.ok(at('f:main.go') > at('f:src/a.go') && at('f:src/a.go') > at('f:empty.go'));
    // Structure keeps its own colors whatever the mode.
    assert.equal(color('d:.', 'size'), 'ta');
  });

  // Verifies: REQ-MAP-037
  it('by history waits for the history, and colors by it once loaded', () => {
    assert.equal(effectiveMode('commits', null), 'language');
    assert.equal(effectiveMode('authors', undefined), 'language');
    assert.equal(effectiveMode('size', null), 'size');
    const hist = { files: { 'main.go': [[200, 0, 5, 1, 0], [100, 1, 3, 0, 1]], 'src/a.go': [[150, 0, 1, 1, 2]] } };
    assert.equal(effectiveMode('commits', hist), 'commits');
    const hm = computeMetrics(model, hist, 0);
    const busy = color('f:main.go', 'commits', { hm });
    assert.equal(busy, seq[seq.length - 1], 'the most changed file is not at the top of the ramp');
    assert.ok(seq.indexOf(color('f:src/a.go', 'commits', { hm })) < seq.indexOf(busy));
    assert.equal(color('f:empty.go', 'commits', { hm }), 'none', 'a file with no history has a color');
  });
});
