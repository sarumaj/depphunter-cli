// Which binary the extension decides to run. No editor and no server needed: this is
// a choice between three things, and getting it wrong means either ignoring what the
// user asked for or starting a server of a different version from the extension.

const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { after, before, describe, it } = require('node:test');

require('./stub'); // binary.js imports vscode, even though it asks it nothing
const { binaryFor, bundled } = require('../out/binary.js');

const NAME = process.platform === 'win32' ? 'depphunter.exe' : 'depphunter';

// Verifies: REQ-EXT-019
describe('which depphunter to run', () => {
  let home, empty;

  before(() => {
    // A stand-in for an installed release: an extension directory with a bin/ in it.
    home = fs.mkdtempSync(path.join(os.tmpdir(), 'dh-home-'));
    fs.mkdirSync(path.join(home, 'bin'));
    fs.writeFileSync(path.join(home, 'bin', NAME), '#!/bin/sh\nexit 0\n', { mode: 0o644 });
    // And one for a build from a checkout, which ships no binary at all.
    empty = fs.mkdtempSync(path.join(os.tmpdir(), 'dh-empty-'));
  });
  after(() => {
    fs.rmSync(home, { recursive: true, force: true });
    fs.rmSync(empty, { recursive: true, force: true });
  });

  it('finds the binary a release ships', () => {
    assert.strictEqual(bundled(home), path.join(home, 'bin', NAME));
  });

  it('finds none in a build that ships none', () => {
    assert.strictEqual(bundled(empty), undefined);
    assert.strictEqual(bundled(undefined), undefined);
  });

  it('makes sure the shipped one can be run', { skip: process.platform === 'win32' ? 'no modes on Windows' : false }, () => {
    // A VSIX is a zip and its permission bits do not survive every installer, so the
    // file above was written without the executable bit on purpose.
    assert.ok(fs.statSync(bundled(home)).mode & 0o111, 'the bundled binary is not executable');
  });

  it('prefers the shipped one over the PATH', () => {
    assert.strictEqual(binaryFor('', home), path.join(home, 'bin', NAME));
    assert.strictEqual(binaryFor(undefined, home), path.join(home, 'bin', NAME));
  });

  it('falls back to the PATH when nothing is shipped', () => {
    assert.strictEqual(binaryFor('', empty), 'depphunter');
  });

  it('lets the setting win over both', () => {
    // The only way to point at a build that is not the one the extension shipped
    // with, so an explicit value is never second-guessed.
    assert.strictEqual(binaryFor('/opt/depphunter', home), '/opt/depphunter');
    assert.strictEqual(binaryFor('  /opt/depphunter  ', home), '/opt/depphunter');
  });
});
