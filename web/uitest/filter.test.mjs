// What the filters leave on the map, and what search can find among it.
//
// A filter is a statement about files - a language, a place in the tree - and the rest
// of the map follows from it: a directory with nothing left in it goes, a package that
// nothing visible imports goes, an island with no packages left sinks. Those knock-on
// rules are where filtering goes wrong, so they are checked here on a small mixed
// repository, along with the globs themselves and the colors a filter must not touch.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { buildModel } = await import('../static/model.js');
const { computeVisibility, globMatcher, parsePathFilter, searchIndex, search } = await import('../static/filter.js');
const { assignSlots, languageColors } = await import('../static/colors.js');

const dir = (path, parent) => ({ id: `d:${path}`, kind: 'dir', name: path.split('/').pop(), path, parent });
const file = (path, parent, lang, loc) => ({ id: `f:${path}`, kind: 'file', name: path.split('/').pop(), path, parent, lang, loc });
const pkg = (eco, name) => ({ id: `p:${eco}:${name}`, kind: 'package', name, parent: `e:${eco}` });
const eco = name => ({ id: `e:${name}`, kind: 'ecosystem', name });
const imports = (from, to) => ({ from: `f:${from}`, to, kind: 'import' });

// Go in src/ (with a testdata directory under it), TypeScript in web/, Python in
// tools/. npm is used only by the TypeScript; golang.org/x/mod by both the real Go
// file and the one in testdata.
const nodes = [
  { id: 'd:.', kind: 'dir', name: '.', path: '.' },
  dir('src', 'd:.'), dir('src/testdata', 'd:src'), dir('web', 'd:.'), dir('tools', 'd:.'),
  file('src/main.go', 'd:src', 'Go', 500),
  file('src/testdata/fix.go', 'd:src/testdata', 'Go', 10),
  file('web/app.ts', 'd:web', 'TypeScript', 300),
  file('web/util.ts', 'd:web', 'TypeScript', 100),
  file('tools/gen.py', 'd:tools', 'Python', 200),
  file('README.md', 'd:.', 'Markdown', 20),
  { id: 's:parseImports', kind: 'symbol', name: 'parseImports', symbolKind: 'func', line: 12, parent: 'f:src/main.go' },
  eco('go'), pkg('go', 'golang.org/x/mod'),
  eco('npm'), pkg('npm', 'react'), pkg('npm', 'lodash'),
  eco('pypi'), pkg('pypi', 'requests'),
];
const edges = [
  imports('src/main.go', 'p:go:golang.org/x/mod'),
  imports('src/testdata/fix.go', 'p:go:golang.org/x/mod'),
  imports('web/app.ts', 'p:npm:react'),
  imports('web/util.ts', 'p:npm:lodash'),
  imports('tools/gen.py', 'p:pypi:requests'),
];
const model = buildModel({ nodes, edges });

/** The ids left visible under these filters, of one kind or all of them. */
function shown({ langs = [], ecos = [], path = '' } = {}, kind) {
  const vis = computeVisibility(model, { hiddenLangs: new Set(langs), hiddenEcosystems: new Set(ecos), path });
  return [...model.byId.values()].filter(n => (!kind || n.kind === kind) && vis.visible(n)).map(n => n.id).sort();
}

describe('filtering the map', () => {
  // Verifies: REQ-MAP-032
  it('hides the files of a hidden language and shows them again', () => {
    assert.deepEqual(shown({ langs: ['TypeScript'] }, 'file'),
      ['f:README.md', 'f:src/main.go', 'f:src/testdata/fix.go', 'f:tools/gen.py']);
    // The symbols of a hidden file go with it.
    assert.equal(shown({ langs: ['Go'] }).includes('s:parseImports'), false);
    // Unhiding is only a smaller set.
    assert.equal(shown({}, 'file').length, 6);
  });

  // Verifies: REQ-MAP-033
  it('sinks a hidden island with all of its packages', () => {
    const left = shown({ ecos: ['e:npm'] });
    for (const id of ['e:npm', 'p:npm:react', 'p:npm:lodash']) assert.ok(!left.includes(id), `${id} survived its island`);
    for (const id of ['e:go', 'e:pypi', 'p:pypi:requests', 'f:web/app.ts']) assert.ok(left.includes(id), `${id} went with npm`);
  });

  // Verifies: REQ-MAP-035
  it('takes a package away with the last visible file that imports it', () => {
    // Only the TypeScript used npm, so without it the whole island goes, and web/ -
    // which had nothing else in it - goes as well.
    const noTs = shown({ langs: ['TypeScript'] });
    for (const id of ['e:npm', 'p:npm:react', 'p:npm:lodash', 'd:web']) assert.ok(!noTs.includes(id), `${id} outlived its importers`);
    assert.ok(noTs.includes('e:go') && noTs.includes('e:pypi'));
    // A package with one importer hidden and one still standing stays, and so does
    // its island.
    const noTestdata = shown({ path: '!**/testdata/**' });
    assert.ok(!noTestdata.includes('f:src/testdata/fix.go'));
    assert.ok(!noTestdata.includes('d:src/testdata'), 'an emptied directory is still on the map');
    assert.ok(noTestdata.includes('p:go:golang.org/x/mod'), 'a package still imported by a visible file was hidden');
    assert.ok(noTestdata.includes('e:go'));
    // ... and goes once the other importer is hidden too.
    assert.ok(!shown({ langs: ['Go'] }).includes('e:go'));
  });

  // Verifies: REQ-MAP-034
  it('keeps what a plain glob names and hides what a ! glob names', () => {
    assert.deepEqual(shown({ path: 'src/**' }, 'file'), ['f:src/main.go', 'f:src/testdata/fix.go']);
    assert.deepEqual(shown({ path: '!**/testdata/**' }, 'file'),
      ['f:README.md', 'f:src/main.go', 'f:tools/gen.py', 'f:web/app.ts', 'f:web/util.ts']);
    assert.deepEqual(shown({ path: '*.go' }, 'file'), ['f:src/main.go', 'f:src/testdata/fix.go']);
    // A list: included by one, then excluded by another.
    assert.deepEqual(shown({ path: 'src/**, !**/testdata/**' }, 'file'), ['f:src/main.go']);
    // Blank entries and a bare ! are nothing, not "match everything" or "hide everything".
    const { include, exclude } = parsePathFilter(' , !, ');
    assert.equal(include.length + exclude.length, 0);
  });

  // Verifies: REQ-MAP-034
  it('reads globs the way gitignore does', () => {
    const m = g => p => globMatcher(g)(p);
    // * and ? stay inside one segment.
    assert.ok(m('src/*.go')('src/a.go'));
    assert.ok(!m('src/*.go')('src/x/a.go'), '* crossed a /');
    assert.ok(m('src/?.go')('src/a.go'));
    assert.ok(!m('src/?.go')('src/ab.go'));
    assert.ok(!m('src/a?b')('src/a/b'), '? crossed a /');
    // ** spans any number of them, none included.
    assert.ok(m('src/**/*.go')('src/a.go'));
    assert.ok(m('src/**/*.go')('src/x/y/z.go'));
    assert.ok(!m('src/**/*.go')('lib/src/a.go'), 'a glob with a / was not anchored');
    // Without a /, a glob is a name that may be any segment.
    assert.ok(m('testdata')('a/testdata/b.go'));
    assert.ok(m('*.go')('deep/down/c.go'));
    assert.ok(!m('testdata')('a/testdatas/b.go'));
    // Regular-expression characters are only themselves.
    assert.ok(m('a+b.go')('a+b.go'));
    assert.ok(!m('a.go')('abgo'));
  });
});

describe('searching the map', () => {
  const finder = searchIndex(model);
  const everything = () => true;

  // Verifies: REQ-MAP-031
  it('finds a symbol by part of its name, with the file it is in', () => {
    const [top] = search(finder, 'parseImp', everything);
    assert.equal(top.node.id, 's:parseImports');
    assert.equal(top.context, 'src/main.go');
  });

  // Verifies: REQ-MAP-031
  it('finds files, directories and packages as well', () => {
    const first = q => search(finder, q, everything)[0]?.node.id;
    assert.equal(first('gen.py'), 'f:tools/gen.py');
    assert.equal(first('testdata/'), 'd:src/testdata');
    assert.equal(first('react'), 'p:npm:react');
    assert.deepEqual(search(finder, '   ', everything), [], 'an empty query listed something');
  });

  // Verifies: REQ-MAP-031
  it('lists only what the filters leave, and no more than twelve', () => {
    const vis = computeVisibility(model, { hiddenLangs: new Set(['Go']), hiddenEcosystems: new Set(), path: '' });
    assert.deepEqual(search(finder, 'parseImp', vis.visible), [], 'a hidden symbol was listed');

    const many = buildModel({
      nodes: [nodes[0], ...Array.from({ length: 30 }, (_, i) => file(`handler${i}.go`, 'd:.', 'Go', 1))],
      edges: [],
    });
    assert.equal(search(searchIndex(many), 'handler', everything).length, 12);
  });
});

describe('language colors', () => {
  const pal = { series: ['c1', 'c2', 'c3', 'c4', 'c5', 'c6', 'c7'], other: 'other' };
  const colorsOf = (m, slots) => {
    const c = languageColors(m, pal, slots);
    return Object.fromEntries(m.languages.map(l => [l.lang, c.of(l.lang)]));
  };

  // Verifies: REQ-MAP-036
  it('gives the slots by lines once, and a filter changes none of them', () => {
    const slots = assignSlots(model);
    assert.deepEqual(slots.slice(0, 4), ['Go', 'TypeScript', 'Python', 'Markdown']);
    const before = colorsOf(model, slots);
    // Hiding the largest language is a statement about what is drawn, not about the
    // repository: the ranking and every other language's color stay where they were.
    computeVisibility(model, { hiddenLangs: new Set(['Go']), hiddenEcosystems: new Set(), path: '' });
    assert.deepEqual(assignSlots(model, slots), slots);
    assert.deepEqual(colorsOf(model, assignSlots(model, slots)), before);
  });

  // Verifies: REQ-MAP-036
  it('keeps every language its color across an update that adds a bigger one', () => {
    const slots = assignSlots(model);
    const before = colorsOf(model, slots);
    const grown = buildModel({ nodes: [...nodes, file('big.rs', 'd:.', 'Rust', 99_000)], edges });
    assert.equal(grown.languages[0].lang, 'Rust', 'the fixture should rank Rust first');
    const after = colorsOf(grown, assignSlots(grown, slots));
    for (const [lang, color] of Object.entries(before)) assert.equal(after[lang], color, `${lang} was recolored`);
    assert.ok(Object.values(before).every(c => c !== after.Rust), 'Rust took a color already in use');

    // A language that leaves frees its slot without moving anybody else's.
    const shrunk = buildModel({ nodes: nodes.filter(n => n.lang !== 'TypeScript'), edges });
    const left = colorsOf(shrunk, assignSlots(shrunk, slots));
    for (const lang of ['Go', 'Python', 'Markdown']) assert.equal(left[lang], before[lang]);
  });
});
