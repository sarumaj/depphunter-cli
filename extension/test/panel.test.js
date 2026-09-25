// The side panel against a real depphunter: the dependency tree it draws, and the
// session state it shares with the map.
//
// What is worth checking here is the part neither the type checker nor the Go tests
// see: that the graph document the server serves turns into rows, that a row picked
// here reaches the server as a selection, and that a backpack changed anywhere else
// reaches the panel over the event stream. All three are the extension and the server
// agreeing on a shape, which is exactly the kind of agreement that rots quietly.
//
// Set DEPPHUNTER to the binary to test against; without one these are skipped.

const assert = require('node:assert');
const { execFileSync } = require('node:child_process');
const http = require('node:http');
const path = require('node:path');
const { after, before, describe, it } = require('node:test');

const stub = require('./stub');
const extension = require('../out/extension.js');

const ROOT = path.join(__dirname, '..', '..');

function available() {
  try {
    execFileSync(stub.settings.path, ['--version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

/** One request to the server under test, with the session token the map carries. */
function call(address, method, apiPath, body) {
  const url = new URL(address);
  const token = url.searchParams.get('token');
  return new Promise((resolve, reject) => {
    const req = http.request({
      hostname: url.hostname, port: url.port, path: apiPath, method,
      headers: {
        'X-Depphunter-Token': token,
        'X-Depphunter-Request': '1',
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
    }, res => {
      const chunks = [];
      res.on('data', c => chunks.push(c));
      res.on('end', () => resolve({ status: res.statusCode, body: Buffer.concat(chunks).toString() }));
    });
    req.on('error', reject);
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

/** Waits for something the event stream has to deliver first. */
async function until(what, check, ms = 5000) {
  const deadline = Date.now() + ms;
  for (;;) {
    const got = check();
    if (got) return got;
    if (Date.now() > deadline) assert.fail(`timed out waiting for ${what}`);
    await new Promise(r => setTimeout(r, 50));
  }
}

const provider = id => stub.last('registerTreeDataProvider', id)?.[2];

describe('the side panel', { skip: available() ? false : 'no depphunter binary (set DEPPHUNTER)' }, () => {
  let address;

  before(async () => {
    stub.setRoot(ROOT);
    stub.settings.args = ['--exclude', 'vendor', '--exclude', 'web/static/vendor', '--no-history'];
    extension.activate({ subscriptions: [] });
    await stub.commands.get('depphunter.open')();
    address = stub.last('panel.html')[1].match(/<iframe src="([^"]+)"/)[1];
  });
  after(() => extension.deactivate());

  // Verifies: REQ-EXT-003
  it('draws the graph as a tree of directories, files and what they import', () => {
    const tree = provider('depphunter.tree');
    assert.ok(tree, 'the Dependencies view has no data provider');
    const roots = tree.getChildren();
    assert.ok(roots.length > 0, 'the tree is empty');

    // The repository's own root is a directory, and it opens into what is in it.
    const dir = roots.find(r => r.node.kind === 'dir');
    assert.ok(dir, `no directory among the roots: ${roots.map(r => r.node.kind)}`);
    const inside = tree.getChildren(dir);
    assert.ok(inside.length > 0, 'the root directory opened into nothing');
    // Directories before files, which is the order a file tree is read in.
    const kinds = inside.map(r => r.node.kind);
    assert.deepStrictEqual(kinds, [...kinds].sort((a, b) => (a === 'dir' ? 0 : 1) - (b === 'dir' ? 0 : 1)));

    // A file opens into what it imports rather than into its symbols: the panel is a
    // dependency tree, and the editor has an outline of its own.
    const files = [];
    const walk = (rows, depth) => {
      for (const row of rows) {
        if (row.node.kind === 'file') files.push(row);
        else if (row.node.kind === 'dir' && depth < 4) walk(tree.getChildren(row), depth + 1);
      }
    };
    walk(inside, 0);
    const importer = files.find(f => tree.getChildren(f).length > 0);
    assert.ok(importer, 'no file in the repository imports anything');
    for (const row of tree.getChildren(importer)) {
      assert.ok(row.node.kind !== 'symbol', `a symbol turned up under a file: ${row.node.id}`);
    }

    const item = tree.getTreeItem(importer);
    assert.strictEqual(item.contextValue, 'file');
    assert.ok(item.command.command === 'depphunter.select');
  });

  // Verifies: REQ-EXT-005
  it('stops a branch that leads back to where it has been', () => {
    const tree = provider('depphunter.tree');
    const eco = tree.getChildren().find(r => r.node.kind === 'ecosystem');
    if (!eco) return; // a repository with no external packages has no cycles either
    const seen = new Set();
    const walk = (row, depth) => {
      if (depth > 12) assert.fail(`the tree went ${depth} deep: a cycle was followed`);
      for (const child of tree.getChildren(row)) {
        seen.add(child.node.id);
        // A repeat is shown once, marked, and opens into nothing.
        if (child.cycle) assert.deepStrictEqual(tree.getChildren(child), []);
        else walk(child, depth + 1);
      }
    };
    walk(eco, 0);
    assert.ok(seen.size > 0, 'the ecosystem held no packages');
  });

  // Verifies: REQ-EXT-007, REQ-SRV-009, REQ-SRV-010
  it('sends a picked row to the map as the selection', async () => {
    const tree = provider('depphunter.tree');
    const row = tree.getChildren().find(r => r.node.kind === 'dir');
    await stub.commands.get('depphunter.select')(row);
    const res = await call(address, 'GET', '/api/session');
    assert.strictEqual(res.status, 200);
    assert.strictEqual(JSON.parse(res.body).selected, row.node.id);
  });

  // Verifies: REQ-EXT-008
  it('opens the tree to what the map selected, and catches up when it was hidden', async () => {
    const tree = provider('depphunter.tree');
    const view = stub.last('createTreeView', 'depphunter.tree')[2];
    const dir = tree.getChildren().find(r => r.node.kind === 'dir');
    const inside = tree.getChildren(dir).find(r => r.node.kind === 'file' || r.node.kind === 'dir');

    // Hidden: nothing is revealed, because there is nothing on screen to reveal in.
    view.visible = false;
    const before = stub.calls.length;
    await call(address, 'POST', '/api/selection', { id: inside.node.id, origin: 'the-map' });
    await until('the selection to arrive', () => stub.calls.length > before || true);
    await new Promise(r => setTimeout(r, 400));
    assert.ok(!stub.last('reveal'), 'revealed into a hidden view');

    // Shown again, it opens to what was picked in the meantime.
    view.visible = true;
    view.visibilityListener({ visible: true });
    const revealed = await until('the tree to catch up', () => stub.last('reveal'));
    assert.strictEqual(revealed[2].node.id, inside.node.id);
    assert.strictEqual(revealed[3].focus, false, 'revealing took the focus');
  });

  // Verifies: REQ-EXT-010, REQ-EXT-011, REQ-SRV-011
  it('shows a backpack changed elsewhere, and takes an item back out of it', async () => {
    const caught = {
      id: 'TEST-1', severity: 'high', title: 'something the scanner said',
      where: 'README.md', line: 1, caughtAt: Date.now(),
    };
    const put = await call(address, 'PUT', '/api/backpack', { items: [caught], origin: 'test' });
    assert.strictEqual(put.status, 204);

    const pack = provider('depphunter.backpack');
    const rows = await until('the backpack to arrive over the event stream',
      () => { const r = pack.getChildren(); return r.length ? r : null; });
    assert.strictEqual(rows[0].id, 'TEST-1');
    const item = pack.getTreeItem(rows[0]);
    assert.strictEqual(item.contextValue, 'finding');
    assert.match(String(item.description), /README\.md:1/);

    // Dropping one here takes it out of the shared backpack, which is the map's too.
    await stub.commands.get('depphunter.dropFinding')(rows[0]);
    const res = await call(address, 'GET', '/api/session');
    assert.deepStrictEqual(JSON.parse(res.body).backpack, []);
    assert.deepStrictEqual(pack.getChildren(), []);
  });

  // Verifies: REQ-EXP-014
  it('exports the backpack in the format that was picked', async () => {
    await call(address, 'PUT', '/api/backpack', {
      items: [{ id: 'TEST-2', severity: 'critical', title: 'a large thing', where: 'a.go' }],
      origin: 'test',
    });
    const res = await call(address, 'GET', '/api/backpack?format=md');
    assert.strictEqual(res.status, 200);
    assert.match(res.body, /- \[ \] \*\*critical\*\* a large thing/);
  });

  // Verifies: REQ-EXT-015
  it('opens the map outside the editor when asked to', async () => {
    await stub.commands.get('depphunter.openExternal')();
    const opened = stub.last('openExternal');
    assert.ok(opened, 'nothing was opened outside the editor');
    assert.match(opened[1], /^http:\/\/127\.0\.0\.1:\d+\/\?token=/);
  });
});
