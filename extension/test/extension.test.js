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

  it('starts a server and shows it in a tab', async () => {
    await stub.commands.get('depphunter.open')();
    const html = stub.last('panel.html');
    assert.ok(html, `the map was not opened: ${JSON.stringify(stub.calls.map(c => c[0]))}`);
    address = html[1].match(/<iframe src="([^"]+)"/)?.[1];
    // The address carries the session token, which is the whole reason it is read
    // from the server's output rather than put together from the port.
    assert.match(address ?? '', ADDRESS);
  });

  it('keeps the token when the address is rewritten under it', async () => {
    // asExternalUri does not reliably keep the query, and in embed mode the query is
    // where the session token is - there is no cookie to hold it inside somebody
    // else's frame. An address that lost it shows "unauthorized" and nothing else.
    await stub.commands.get('depphunter.stop')();
    stub.settings.dropQuery = true;
    try {
      await stub.commands.get('depphunter.open')();
      const html = stub.last('panel.html')[1];
      const shown = html.match(/<iframe src="([^"]+)"/)?.[1];
      assert.match(shown ?? '', ADDRESS, 'the token did not survive the rewrite');
      assert.strictEqual((await fetch(shown)).status, 200);
    } finally {
      stub.settings.dropQuery = false;
    }
  });

  it('leaves the frame the pointer lock the editor granted it', async () => {
    // A nested frame already carries every restriction its ancestors carry, so a
    // sandbox attribute here can only take something away - and what it takes away
    // is walk mode's mouse. This is what the built-in browser does, and the reason
    // the map is not shown there any more.
    const html = stub.last('panel.html')[1];
    assert.ok(!/<iframe[^>]*\bsandbox\b/.test(html), `the iframe is sandboxed:\n${html}`);
  });

  it('uses the built-in browser when asked to', async () => {
    await stub.commands.get('depphunter.stop')();
    stub.settings.openIn = 'simpleBrowser';
    try {
      await stub.commands.get('depphunter.open')();
      const opened = stub.last('executeCommand', 'simpleBrowser.show');
      assert.ok(opened, 'the built-in browser was not asked to show anything');
      assert.match(opened[2], ADDRESS);
      address = opened[2];
    } finally {
      stub.settings.openIn = 'webview';
    }
  });

  it('serves the map at the address it handed over', async () => {
    const res = await fetch(address);
    assert.strictEqual(res.status, 200);
    // --embed, without which the editor's browser could not show the page at all.
    const csp = res.headers.get('content-security-policy');
    assert.match(csp, /frame-ancestors /);
    // Every frame above the page has to be named, not only the one holding it: the
    // built-in browser is the editor's window framing a webview framing the page
    // that frames the map, and a single origin missing from this list is an empty
    // tab with the reason buried in the webview's developer tools.
    for (const origin of ['vscode-webview:', 'vscode-file:', 'https://*.vscode-cdn.net']) {
      assert.ok(csp.includes(origin), `frame-ancestors is missing ${origin}: ${csp}`);
    }
  });

  it('reuses the server rather than starting a second one', async () => {
    await stub.commands.get('depphunter.open')();
    assert.strictEqual(stub.last('executeCommand', 'simpleBrowser.show')[2], address);
  });

  it('closes the tab when the server stops', async () => {
    stub.settings.openIn = 'webview';
    await stub.commands.get('depphunter.open')();
    assert.ok(stub.last('panel.html'), 'no tab was opened');
    await stub.commands.get('depphunter.stop')();
    assert.ok(stub.last('panel.dispose'), 'the tab outlived the server');
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
