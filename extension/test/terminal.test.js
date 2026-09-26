// The binary on the PATH of the editor's terminals. No editor needed: which folder
// goes on PATH is a question about the binary the extension would run, and what is
// done with it is a handful of calls on the collection the editor hands over.

const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { after, before, describe, it } = require('node:test');

require('./stub'); // terminal.js imports binary.js, which imports vscode
const { binDirFor, exposeOnPath } = require('../out/terminal.js');

const NAME = process.platform === 'win32' ? 'depphunter.exe' : 'depphunter';

/** The editor's environment collection, as far as the extension uses it. */
function collection() {
  const vars = new Map();
  return {
    vars,
    clear() { vars.clear(); },
    prepend(name, value) { vars.set(name, { type: 'prepend', value }); },
  };
}

// Verifies: REQ-EXT-033
describe('the binary on the terminals\' PATH', () => {
  let home, empty;

  before(() => {
    // An installed release, with its binary in bin/, and a build that ships none.
    home = fs.mkdtempSync(path.join(os.tmpdir(), 'dh-home-'));
    fs.mkdirSync(path.join(home, 'bin'));
    fs.writeFileSync(path.join(home, 'bin', NAME), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
    empty = fs.mkdtempSync(path.join(os.tmpdir(), 'dh-empty-'));
  });
  after(() => {
    fs.rmSync(home, { recursive: true, force: true });
    fs.rmSync(empty, { recursive: true, force: true });
  });

  it('puts the folder of the binary the extension runs on it', () => {
    assert.strictEqual(binDirFor('', home), path.join(home, 'bin'), 'the bundled binary');
    const own = path.join(empty, 'build', NAME);
    assert.strictEqual(binDirFor(own, home), path.dirname(own), 'the one depphunter.path names');
  });

  it('adds nothing where the shell finds the binary anyway, or when switched off', () => {
    assert.strictEqual(binDirFor('', empty), undefined, 'a build without a binary falls back to PATH');
    assert.strictEqual(binDirFor('depphunter-dev', home), undefined, 'a bare name is looked up on PATH');
    assert.strictEqual(binDirFor('', home, false), undefined, 'depphunter.addToPath off');
  });

  it('puts it first, and replaces what it put there before', () => {
    const env = collection();
    exposeOnPath(env, path.join(home, 'bin'));
    assert.deepStrictEqual(env.vars.get('PATH'), { type: 'prepend', value: path.join(home, 'bin') + path.delimiter });
    assert.match(env.description, /depphunter/);

    exposeOnPath(env, '/elsewhere');
    assert.deepStrictEqual([...env.vars.keys()], ['PATH']);
    assert.strictEqual(env.vars.get('PATH').value, '/elsewhere' + path.delimiter, 'the old folder is still there');

    exposeOnPath(env, undefined);
    assert.strictEqual(env.vars.size, 0, 'switching it off left PATH changed');
  });
});
