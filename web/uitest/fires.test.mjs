// What burns, where it goes, and what putting it out means.
//
// The fire is the one thing on this map that changes while nobody touches it, so the
// rules had better be the ones that were meant. Two of them carry the whole analogy
// and neither is visible in a screenshot: fire only starts where a scanner proved the
// vulnerable code can be reached, and it travels against the imports - to whatever
// calls the burning file - because that is the direction reachability actually runs.
// A fire that spread the other way would be telling the reader something false about
// their own code, very persuasively.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import './stub.mjs';

const { Fires, seatsOf, reachable, MOST, CATCHES, SPREADS_AT, SPREADS_EVERY, COOLS_FOR, DOUSES } =
  await import('../static/fires.js');
const { buildModel } = await import('../static/model.js');

const vuln = (o = {}) => ({
  id: o.id || 'GO-1', kind: 'vulnerability', severity: 'high', title: 'bad',
  ecosystem: 'go', package: 'evil.dev/pkg', reached: 'app.Handler', path: 'a.go', ...o,
});

/** A repository of files importing one another, plus one package they pull in. */
function world({ files = ['a.go', 'b.go', 'c.go'], imports = [], pkgs = ['evil.dev/pkg'] } = {}) {
  const nodes = [{ id: 'd:.', kind: 'dir', name: '.', path: '.' }];
  for (const f of files) nodes.push({ id: `f:${f}`, kind: 'file', name: f, path: f, parent: 'd:.', loc: 10 });
  nodes.push({ id: 'e:go', kind: 'ecosystem', name: 'go' });
  for (const p of pkgs) nodes.push({ id: `p:go:${p}`, kind: 'package', name: p, parent: 'e:go' });
  const edges = imports.map(([from, to]) => ({ from, to, kind: 'import' }));
  return buildModel({ nodes, edges });
}

const index = (...findings) => ({ all: findings });

/** Run the clock, a frame at a time, so spreading gets the ticks it asks for. */
const run = (fires, seconds, step = 0.25) => {
  for (let t = 0; t < seconds; t += step) fires.step(step);
};

describe('what catches fire', () => {
  it('burns only what a scanner proved is reachable', () => {
    assert.equal(reachable(vuln()), true);
    // The common case by far: the dependency is vulnerable, nothing here calls in.
    assert.equal(reachable(vuln({ reached: '' })), false);
    assert.equal(reachable(vuln({ reached: undefined })), false);
    // A linter's objection is not a fire however bad it is.
    assert.equal(reachable({ kind: 'lint', severity: 'critical', reached: 'x' }), false);
  });

  it('seats a fire on the package and on the call site', () => {
    const m = world();
    const [seat] = seatsOf(index(vuln()), m);
    assert.equal(seat.from, 'p:go:evil.dev/pkg', 'the fire does not start at the package');
    assert.equal(seat.into, 'f:a.go', 'the call site is not where it is going');
  });

  it('drops a finding with neither end on the map', () => {
    const m = world({ files: ['a.go'], pkgs: [] });
    // A vendored path and a package the map does not draw: nowhere to put it.
    assert.deepEqual(seatsOf(index(vuln({ path: 'vendor/x.go' })), m), []);
    // One end is enough, though.
    assert.equal(seatsOf(index(vuln({ package: '', path: 'a.go' })), m).length, 1);
  });

  it('ignores what is not reachable, however bad', () => {
    const m = world();
    assert.deepEqual(seatsOf(index(vuln({ severity: 'critical', reached: '' })), m), []);
  });
});

describe('where the fire goes', () => {
  it('travels from the package to the call site the scanner named', () => {
    const m = world();
    const fires = new Fires(m);
    fires.light(index(vuln()));
    assert.deepEqual([...fires.lit.keys()], ['p:go:evil.dev/pkg']);
    run(fires, SPREADS_EVERY + 2 / CATCHES);
    assert.ok(fires.lit.has('f:a.go'), 'the fire never reached the call site');
  });

  it('travels against the imports, to whatever calls the burning file', () => {
    // b imports a, c imports b. A vulnerability reachable from a is reachable from
    // both, and the fire has to say so in that order.
    const m = world({ imports: [['f:b.go', 'f:a.go'], ['f:c.go', 'f:b.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY * 4 + 8);
    assert.ok(fires.lit.has('f:b.go'), 'the fire did not reach the file that imports the call site');
    assert.ok(fires.lit.has('f:c.go'), 'the fire stopped one hop short');
  });

  it('never goes the way the arrows point', () => {
    // a imports b. Nothing about a burning makes b reachable, and a fire that ran
    // down the arrow would be blaming a file for its own dependency's problem.
    const m = world({ files: ['a.go', 'b.go'], imports: [['f:a.go', 'f:b.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY * 4 + 8);
    assert.equal(fires.lit.has('f:b.go'), false, 'the fire ran down an import');
  });

  it('never sets a package alight from the repository', () => {
    // c imports the package directly as well. Nothing this project does can light a
    // dependency: the fire only ever comes out of one.
    const m = world({ imports: [['f:b.go', 'f:a.go'], ['f:b.go', 'p:go:evil.dev/pkg']], pkgs: ['evil.dev/pkg', 'other.dev/p'] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY * 5 + 10);
    assert.equal(fires.lit.has('p:go:other.dev/p'), false, 'a second package caught');
    for (const [id] of fires.lit) {
      if (id === 'p:go:evil.dev/pkg') continue;
      assert.ok(id.startsWith('f:'), `${id} is not a file and is alight`);
    }
  });

  it('waits until a building is well alight before passing it on', () => {
    const m = world({ imports: [['f:b.go', 'f:a.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    // One tick: the package is lit but nowhere near hot enough to pass it on.
    fires.step(0.1);
    assert.equal(fires.lit.size, 1, 'the fire crossed the map in a frame');
    assert.ok(fires.lit.get('p:go:evil.dev/pkg').heat < SPREADS_AT);
  });

  it('stops at what the eye can read', () => {
    const files = Array.from({ length: MOST + 40 }, (_, i) => `f${i}.go`);
    // A chain: each file imports the one before it, so the fire has somewhere to go
    // for as long as it likes.
    const imports = files.slice(1).map((f, i) => [`f:${f}`, `f:${files[i]}`]);
    const m = world({ files, imports });
    const fires = new Fires(m);
    fires.light(index(vuln({ path: 'f0.go' })));
    run(fires, SPREADS_EVERY * (MOST + 60));
    assert.ok(fires.lit.size <= MOST, `${fires.lit.size} buildings alight, over the cap of ${MOST}`);
  });
});

describe('putting it out', () => {
  it('reports what one moment of spray did', () => {
    const m = world();
    const fires = new Fires(m);
    fires.light(index(vuln()));
    assert.equal(fires.douse('f:nothing.go', 1), null, 'doused something that was not alight');
    assert.equal(fires.douse('p:go:evil.dev/pkg', 0.01), 'cooling');
    assert.equal(fires.douse('p:go:evil.dev/pkg', 10), 'out');
  });

  it('puts out everything a fire lit when the source goes out', () => {
    // The lesson: hold the line on one building and the fire keeps arriving behind
    // it; upgrade the dependency and all of it goes out at once.
    const m = world({ imports: [['f:b.go', 'f:a.go'], ['f:c.go', 'f:b.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY * 4 + 8);
    assert.ok(fires.lit.size > 1, 'the fire never spread, so there is nothing to test');
    assert.equal(fires.douse('p:go:evil.dev/pkg', 10), 'out');
    assert.equal(fires.burning, 0, 'the fire outlived the package it came from');
    assert.deepEqual(fires.doused, ['GO-1']);
  });

  it('holds a line without ending the fire', () => {
    const m = world({ imports: [['f:b.go', 'f:a.go'], ['f:c.go', 'f:b.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY * 3 + 6);
    assert.ok(fires.lit.has('f:a.go'));
    assert.equal(fires.douse('f:a.go', 10), 'cooled', 'a building away from the source ended the fire');
    assert.ok(fires.burning > 0, 'the source went out with a building that was not it');
  });

  it('keeps a doused building cold for a while', () => {
    const m = world({ imports: [['f:b.go', 'f:a.go']] });
    const fires = new Fires(m);
    fires.light(index(vuln()));
    run(fires, SPREADS_EVERY + 6);
    fires.douse('f:a.go', 10);
    run(fires, SPREADS_EVERY * 2);
    assert.equal(fires.lit.has('f:a.go'), false, 'it relit while the walker was still standing there');
    run(fires, COOLS_FOR + SPREADS_EVERY * 2);
    assert.ok(fires.cooled.has('f:a.go') === false, 'it stayed cold for good');
  });

  it('does not light a fire the backpack remembers putting out', () => {
    const m = world();
    const fires = new Fires(m);
    fires.keepDoused(['GO-1']);
    fires.light(index(vuln()));
    assert.equal(fires.burning, 0, 'a fire already put out came back on a relayout');
  });
});
