// The extension against a real depphunter, with the editor stubbed out (test/stub.js).
//
// Set DEPPHUNTER to the binary to test against; without one the tests are skipped,
// because there is nothing here that can stand in for the server.

const assert = require('node:assert');
const { execFileSync } = require('node:child_process');
const path = require('node:path');
const { after, before, describe, it } = require('node:test');

const stub = require('./stub');
const extension = require('../out/extension.js');

const ROOT = path.join(__dirname, '..', '..'); // the repository: something real to map
const ADDRESS = /^http:\/\/127\.0\.0\.1:\d+\/\?token=[0-9a-f]{48}$/;

function available() {
  try {
    execFileSync(stub.settings.path, ['--version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

describe('depphunter.open', { skip: available() ? false : 'no depphunter binary (set DEPPHUNTER)' }, () => {
  before(() => {
    stub.setRoot(ROOT);
    stub.settings.args = ['--exclude', 'vendor', '--no-history'];
    extension.activate({ subscriptions: [] });
  });
  after(() => extension.deactivate());

  let address;

  it('starts a server and shows it in the built-in browser', async () => {
    await stub.commands.get('depphunter.open')();
    const opened = stub.last('executeCommand', 'simpleBrowser.show');
    assert.ok(opened, `the map was not opened: ${JSON.stringify(stub.calls)}`);
    address = opened[2];
    // The address carries the session token, which is the whole reason it is read
    // from the server's output rather than put together from the port.
    assert.match(address, ADDRESS);
  });

  it('serves the map at the address it handed over', async () => {
    const res = await fetch(address);
    assert.strictEqual(res.status, 200);
    // --embed, without which the editor's browser could not show the page at all.
    assert.match(res.headers.get('content-security-policy'), /frame-ancestors vscode-webview:/);
  });

  it('reuses the server rather than starting a second one', async () => {
    await stub.commands.get('depphunter.open')();
    assert.strictEqual(stub.last('executeCommand', 'simpleBrowser.show')[2], address);
  });

  it('stops the server when told to', async () => {
    await stub.commands.get('depphunter.stop')();
    await new Promise(r => setTimeout(r, 1000));
    const answered = await fetch(address).then(() => true, () => false);
    assert.strictEqual(answered, false, 'the server outlived the stop command');
  });

  it('reports a binary it cannot find', async () => {
    const real = stub.settings.path;
    stub.settings.path = path.join(__dirname, 'no-such-depphunter');
    try {
      await stub.commands.get('depphunter.open')();
      assert.match(stub.last('error')[1], /was not found/);
    } finally {
      stub.settings.path = real;
    }
  });
});
