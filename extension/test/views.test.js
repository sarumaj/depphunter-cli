// The side panel, the commands and the exports against a stand-in server.
//
// No depphunter is needed: server.js looks spawn up on node:child_process each time
// it starts one, so spawn is swapped here for a fake child whose "server" is a
// node:http one in this process. It prints the line the real one prints, answers the
// few API calls the extension makes, and records each of them - which is what these
// tests are about: which requests the extension makes, what it sends, and what it
// does with the answers. What the real server answers is panel.test.js's business.

const assert = require('node:assert');
const childProcess = require('node:child_process');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { PassThrough } = require('node:stream');
const { after, before, describe, it } = require('node:test');

const stub = require('./stub');
const extension = require('../out/extension.js');
const { Api } = require('../out/api.js');
const { DependencyTree } = require('../out/tree.js');

const MANIFEST = require('../../package.json');

/** A small graph: a directory holding a file that imports a package with a dependency. */
function graphOf(name, extra = []) {
  return {
    root: name, generatedAt: '2026-01-01T00:00:00Z',
    nodes: [
      { id: `${name}`, kind: 'dir', name },
      { id: `${name}/a.go`, kind: 'file', name: 'a.go', parent: `${name}`, path: 'a.go', loc: 3 },
      { id: 'eco:go', kind: 'ecosystem', name: 'go' },
      { id: 'pkg:x', kind: 'package', name: 'x', parent: 'eco:go', version: '1.0.0' },
      { id: 'pkg:y', kind: 'package', name: 'y', parent: 'eco:go', version: '2.0.0' },
      ...extra,
    ],
    edges: [
      { from: `${name}/a.go`, to: 'pkg:x', kind: 'import' },
      { from: 'pkg:x', to: 'pkg:y', kind: 'depends' },
      // A cycle, so that expanding "everything" still ends.
      { from: 'pkg:y', to: 'pkg:x', kind: 'depends' },
    ],
  };
}

/**
 * What stands in for one depphunter server: the API the extension uses, with every
 * request recorded and an event stream the test can announce on.
 */
class Fake {
  constructor(root) {
    this.root = root;
    this.name = path.basename(root);
    this.graph = graphOf(this.name);
    this.version = 1;
    this.selected = '';
    this.backpack = [{ id: `${this.name}-F1`, severity: 'high', title: `finding in ${this.name}`, where: 'a.go', nodeId: `${this.name}/a.go` }];
    this.requests = [];
    this.streams = [];
    this.server = http.createServer((req, res) => this.handle(req, res));
  }

  get etag() { return `"g${this.version}"`; }

  /** The graph changes, as a --watch re-analysis would change it. */
  change(graph) {
    this.graph = graph;
    this.version++;
  }

  announce(name, data) {
    for (const res of this.streams) res.write(`event: ${name}\ndata: ${JSON.stringify(data)}\n\n`);
  }

  /** The recorded API requests, the event stream and the page itself left out. */
  calls() {
    return this.requests.filter(r => r.path.startsWith('/api/') && r.path !== '/api/events');
  }

  handle(req, res) {
    const chunks = [];
    req.on('data', c => chunks.push(c));
    req.on('end', () => {
      const url = new URL(req.url, 'http://x');
      const text = Buffer.concat(chunks).toString();
      const body = text ? JSON.parse(text) : undefined;
      this.requests.push({ method: req.method, path: url.pathname, query: url.search, headers: req.headers, body });
      const json = v => { res.writeHead(200, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(v)); };
      switch (`${req.method} ${url.pathname}`) {
        case 'GET /':
          res.end('<!doctype html>');
          return;
        case 'GET /api/events':
          res.writeHead(200, { 'Content-Type': 'text/event-stream' });
          res.write(`event: hello\ndata: ${JSON.stringify({ version: this.version, etag: this.etag, seq: 0, resumed: false })}\n\n`);
          this.streams.push(res);
          return;
        case 'GET /api/graph':
          if (req.headers['if-none-match'] === this.etag) {
            res.writeHead(304, { ETag: this.etag });
            res.end();
            return;
          }
          res.setHeader('ETag', this.etag);
          json(this.graph);
          return;
        case 'GET /api/session':
          json({ selected: this.selected, backpack: this.backpack });
          return;
        case 'POST /api/selection':
          // The real server hands a selection to every client, its sender included.
          this.selected = body.id;
          res.writeHead(204).end();
          this.announce('selection', body);
          return;
        case 'PUT /api/backpack':
          this.backpack = body.items;
          res.writeHead(204).end();
          this.announce('backpack', { count: body.items.length, origin: body.origin });
          return;
        case 'GET /api/backpack':
          res.end(`backpack of ${this.name} as ${url.searchParams.get('format')}`);
          return;
        case 'GET /api/export':
          res.end(`graph of ${this.name} as ${url.searchParams.get('format')}`);
          return;
        default:
          res.writeHead(404).end();
      }
    });
  }
}

/** The fake servers started so far, by the folder they were started for. */
const fakes = new Map();
const spawned = [];

const realSpawn = childProcess.spawn;
function fakeSpawn(bin, args) {
  const root = args.at(-1);
  spawned.push(args);
  const fake = new Fake(root);
  fakes.set(root, fake);
  const child = new EventEmitter();
  child.stdout = new PassThrough();
  child.stderr = new PassThrough();
  child.kill = () => {
    for (const res of fake.streams) res.destroy();
    fake.server.closeAllConnections?.();
    fake.server.close();
    setImmediate(() => child.emit('exit', null, 'SIGTERM'));
    return true;
  };
  fake.server.listen(0, '127.0.0.1', () => {
    child.stdout.write(`level=INFO msg="serving at http://127.0.0.1:${fake.server.address().port}/?token=${'0'.repeat(48)}"\n`);
  });
  return child;
}

/** Waits for something that arrives over the event stream or a request. */
async function until(what, check, ms = 3000) {
  const deadline = Date.now() + ms;
  for (;;) {
    const got = check();
    if (got) return got;
    if (Date.now() > deadline) assert.fail(`timed out waiting for ${what}`);
    await new Promise(r => setTimeout(r, 20));
  }
}

const settle = (ms = 150) => new Promise(r => setTimeout(r, ms));
const provider = id => stub.last('registerTreeDataProvider', id)?.[2];
const treeView = () => stub.last('createTreeView', 'depphunter.tree')[2];
const open = dir => stub.commands.get('depphunter.open')(stub.vscode.Uri.file(dir));
const stop = dir => stub.commands.get('depphunter.stop')(stub.vscode.Uri.file(dir));
const since = (mark, kind) => stub.calls.slice(mark).filter(c => c[0] === kind);

/** Every row of the tree, opened all the way down. */
function expandAll(tree) {
  const rows = [];
  const walk = list => {
    for (const row of list) {
      rows.push(row);
      tree.getTreeItem(row);
      walk(tree.getChildren(row));
    }
  };
  walk(tree.getChildren());
  return rows;
}

describe('the panel and the commands, against a stand-in server', () => {
  let base, first, second, nested;

  before(() => {
    base = fs.mkdtempSync(path.join(os.tmpdir(), 'dh-views-'));
    first = path.join(base, 'alpha');
    second = path.join(base, 'beta');
    nested = path.join(first, 'sub');
    for (const dir of [first, second, nested]) fs.mkdirSync(dir, { recursive: true });
    stub.setRoot(first);
    childProcess.spawn = fakeSpawn;
    extension.activate({ subscriptions: [] });
  });
  after(() => {
    extension.deactivate();
    childProcess.spawn = realSpawn;
    fs.rmSync(base, { recursive: true, force: true });
  });

  // Verifies: REQ-EXT-001
  it('contributes a container of its own with Maps, Dependencies and Backpack, each with a provider', () => {
    // The manifest is read, not changed: it is what the editor draws the activity bar from.
    const container = MANIFEST.contributes.viewsContainers.activitybar.find(c => c.id === 'depphunter');
    assert.ok(container, 'no activity-bar container named depphunter');
    assert.ok(fs.existsSync(path.join(__dirname, '..', '..', container.icon)), `its icon is missing: ${container.icon}`);
    const views = MANIFEST.contributes.views.depphunter;
    assert.deepStrictEqual(views.map(v => [v.id, v.name]), [
      ['depphunter.maps', 'Maps'],
      ['depphunter.tree', 'Dependencies'],
      ['depphunter.backpack', 'Backpack'],
    ]);
    // A view without a provider is a view that says "no data provider registered".
    for (const { id } of views) {
      const p = provider(id);
      assert.ok(p, `${id} has no data provider after activation`);
      assert.strictEqual(typeof p.getChildren, 'function');
      assert.strictEqual(typeof p.getTreeItem, 'function');
    }
  });

  // Verifies: REQ-EXT-027
  it('registers every command it contributes, and offers Open the Map on folders in the explorer', () => {
    const contributed = MANIFEST.contributes.commands;
    // Registered both ways round: a contributed command nobody registered is an
    // error when picked, and a registered one nobody contributed cannot be picked.
    assert.deepStrictEqual(contributed.map(c => c.command).sort(), [...stub.commands.keys()].sort());
    for (const c of contributed) {
      assert.match(c.command, /^depphunter\./);
      assert.strictEqual(c.category, 'depphunter', `${c.command} is not titled "depphunter: ..."`);
    }
    const titles = new Map(contributed.map(c => [c.title, c.command]));
    for (const title of [
      'Open the Map', 'Open the Map in the Browser', 'Restart the Server', 'Stop the Server',
      'Show the Server Log', 'Show the Resolution Report', 'Export the Graph', 'Export the Backpack',
      'Open Settings',
    ]) {
      assert.ok(titles.has(title), `no command titled "depphunter: ${title}"`);
    }
    const hidden = new Set((MANIFEST.contributes.menus.commandPalette ?? [])
      .filter(m => m.when === 'false').map(m => m.command));
    for (const title of ['Open the Map', 'Export the Graph', 'Export the Backpack']) {
      assert.ok(!hidden.has(titles.get(title)), `${title} is hidden from the palette`);
    }
    const explorer = MANIFEST.contributes.menus['explorer/context'];
    assert.ok(explorer.some(m => m.command === 'depphunter.open' && m.when === 'explorerResourceIsFolder'),
      `Open the Map is not on the explorer's folder menu: ${JSON.stringify(explorer)}`);
  });

  // Verifies: REQ-EXT-013, REQ-EXT-014
  it('says there is nothing to export before a map is open', async () => {
    for (const command of ['depphunter.export', 'depphunter.exportBackpack']) {
      const mark = stub.calls.length;
      await stub.commands.get(command)();
      assert.match(since(mark, 'info').at(-1)?.[1] ?? '', /nothing to export/, `${command} said nothing`);
      assert.strictEqual(since(mark, 'showSaveDialog').length, 0, `${command} asked where to save nothing`);
    }
  });

  // Verifies: REQ-EXT-027
  it('maps a folder inside a workspace folder when it is the one picked in the explorer', async () => {
    await open(nested);
    assert.strictEqual(spawned.at(-1).at(-1), nested, 'the server was started for another folder');
    assert.ok(provider('depphunter.maps').getChildren().some(u => u.fsPath === nested),
      'the Maps view does not list the folder that was mapped');
    await stop(nested);
  });

  // Verifies: REQ-EXT-027
  it('asks which folder to map when the palette is used with several open', async () => {
    const workspace = stub.vscode.workspace;
    const window = stub.vscode.window;
    const folders = Object.getOwnPropertyDescriptor(workspace, 'workspaceFolders');
    const ask = window.showWorkspaceFolderPick;
    let asked = 0;
    Object.defineProperty(workspace, 'workspaceFolders', {
      configurable: true,
      get: () => [first, second].map(p => ({ uri: stub.vscode.Uri.file(p), name: path.basename(p) })),
    });
    window.showWorkspaceFolderPick = async () => { asked++; return workspace.workspaceFolders[1]; };
    try {
      await stub.commands.get('depphunter.open')();
      assert.strictEqual(asked, 1, 'the user was not asked which folder');
      assert.strictEqual(spawned.at(-1).at(-1), second, 'the folder picked was not the one mapped');
    } finally {
      Object.defineProperty(workspace, 'workspaceFolders', folders);
      window.showWorkspaceFolderPick = ask;
      await stop(second);
    }
  });

  // Verifies: REQ-EXT-002
  it('shows the map opened last, and empties when its server stops', async () => {
    const tree = provider('depphunter.tree');
    const pack = provider('depphunter.backpack');

    await open(first);
    assert.strictEqual(treeView().title, 'Dependencies: alpha');
    assert.ok(tree.getChildren().some(r => r.node.id === 'alpha'), 'the tree is not alpha\'s graph');
    assert.deepStrictEqual(pack.getChildren().map(i => i.id), ['alpha-F1']);

    // A second folder: both views move over to it, although alpha keeps running.
    await open(second);
    assert.strictEqual(treeView().title, 'Dependencies: beta');
    const roots = tree.getChildren().map(r => r.node.id);
    assert.ok(roots.includes('beta') && !roots.includes('alpha'), `the tree still shows alpha: ${roots}`);
    assert.deepStrictEqual(pack.getChildren().map(i => i.id), ['beta-F1']);

    // Stopping the one not on show leaves the views alone...
    await stop(first);
    assert.strictEqual(treeView().title, 'Dependencies: beta');
    assert.ok(tree.getChildren().length > 0);

    // ...and stopping the one on show empties both.
    await stop(second);
    assert.deepStrictEqual(tree.getChildren(), []);
    assert.deepStrictEqual(pack.getChildren(), []);
    assert.strictEqual(treeView().title, 'Dependencies');
  });

  // Verifies: REQ-EXT-004
  it('opens rows without asking the server, and reads the graph again only when told to', async () => {
    await open(first);
    const fake = fakes.get(first);
    const tree = provider('depphunter.tree');
    const graphReads = () => fake.calls().filter(r => r.path === '/api/graph');
    assert.strictEqual(graphReads().length, 1, 'attaching read the graph more than once');
    await until('the event stream to connect', () => fake.streams.length > 0);
    await settle();

    // Opening everything, twice over, is all answered from the graph already here.
    const before = fake.calls().length;
    expandAll(tree);
    expandAll(tree);
    await settle();
    assert.deepStrictEqual(fake.calls().slice(before), [], 'expanding rows made requests');

    // A graph announcement: one conditional read, and the new graph drawn.
    fake.change(graphOf('alpha', [{ id: 'pkg:z', kind: 'package', name: 'z', parent: 'eco:go' }]));
    fake.announce('graph', { version: fake.version });
    await until('the graph to be read again', () => graphReads().length === 2);
    await settle();
    assert.strictEqual(graphReads().length, 2, 'one announcement read the graph more than once');
    assert.strictEqual(graphReads()[1].headers['if-none-match'], '"g1"', 'the read was not conditional');
    const eco = tree.getChildren().find(r => r.node.kind === 'ecosystem');
    assert.ok(tree.getChildren(eco).some(r => r.node.id === 'pkg:z'), 'the new graph was not drawn');

    // Announced again without a change: the server says 304 and the tree is left be.
    const model = tree.graphModel;
    fake.announce('graph', { version: fake.version });
    await until('the graph to be asked about again', () => graphReads().length === 3);
    await settle();
    assert.strictEqual(graphReads()[2].headers['if-none-match'], fake.etag);
    assert.strictEqual(tree.graphModel, model, 'an unchanged graph was rebuilt');

    // Refresh is somebody asking for it: read whatever the server thinks.
    await stub.commands.get('depphunter.refresh')();
    await until('Refresh to read the graph', () => graphReads().length === 4);
    assert.strictEqual(graphReads()[3].headers['if-none-match'], undefined, 'Refresh read conditionally');
  });

  // Verifies: REQ-EXT-009
  it('names itself in what it sends, and does not re-apply its own selection', async () => {
    const fake = fakes.get(first);
    const tree = provider('depphunter.tree');
    const dir = tree.getChildren().find(r => r.node.kind === 'dir');
    const file = tree.getChildren(dir)[0];

    const mark = stub.calls.length;
    await stub.commands.get('depphunter.select')(file);
    const sent = fake.calls().filter(r => r.path === '/api/selection').at(-1);
    assert.strictEqual(sent.body.id, file.node.id);
    const origin = sent.body.origin;
    assert.match(origin ?? '', /\S/, 'the selection carried no origin');
    // The server hands it straight back on the event stream; this client made it,
    // so it is not revealed a second time.
    await settle(300);
    assert.deepStrictEqual(since(mark, 'reveal'), [], 'our own selection was applied again');

    // The same selection from somebody else is revealed.
    fake.announce('selection', { id: file.node.id, origin: 'the-map' });
    const revealed = await until('a selection from elsewhere to be revealed', () => since(mark, 'reveal').at(-1));
    assert.strictEqual(revealed[2].node.id, file.node.id);

    // The backpack goes back with the same name on it.
    const [item] = provider('depphunter.backpack').getChildren();
    await stub.commands.get('depphunter.dropFinding')(item);
    const put = fake.calls().filter(r => r.path === '/api/backpack' && r.method === 'PUT').at(-1);
    assert.strictEqual(put.body.origin, origin);
    // And the name is this instance's: another client in the same host is another name.
    assert.notStrictEqual(new Api('http://127.0.0.1:1/?token=t').origin, origin);
  });

  // Verifies: REQ-EXT-012
  it('selects the node a backpack entry belongs to', async () => {
    const fake = fakes.get(first);
    const pack = provider('depphunter.backpack');
    fake.backpack = [
      { id: 'F2', severity: 'critical', title: 'in the file', where: 'a.go', nodeId: 'alpha/a.go' },
      { id: 'F3', severity: 'low', title: 'nowhere in particular', where: '' },
    ];
    fake.announce('backpack', { count: 2, origin: 'the-map' });
    const rows = await until('the backpack to arrive', () => (pack.getChildren().length === 2 ? pack.getChildren() : null));
    const item = pack.getTreeItem(rows[0]);
    assert.strictEqual(item.command.command, 'depphunter.showFinding', 'picking an entry does nothing');

    const mark = stub.calls.length;
    const before = fake.calls().length;
    await stub.commands.get(item.command.command)(...item.command.arguments);
    const sent = fake.calls().slice(before).filter(r => r.path === '/api/selection');
    assert.deepStrictEqual(sent.map(r => r.body.id), ['alpha/a.go'], 'the map was not told');
    const revealed = since(mark, 'reveal').at(-1);
    assert.ok(revealed, 'the tree was not opened to it');
    assert.strictEqual(revealed[2].node.id, 'alpha/a.go');

    // An entry that belongs to no node has nothing to select.
    const quiet = fake.calls().length;
    await stub.commands.get('depphunter.showFinding')(rows[1]);
    assert.deepStrictEqual(fake.calls().slice(quiet), []);
  });

  /** Runs an export command once per format, picking it and saving where it suggests. */
  async function exportEach(command, expected, suffix, endpoint) {
    const fake = fakes.get(first);
    const window = stub.vscode.window;
    const pick = window.showQuickPick;
    const save = window.showSaveDialog;
    try {
      for (const [label, format, ext] of expected) {
        let offered;
        let suggested;
        window.showQuickPick = async items => { offered = items.map(i => i.label); return items.find(i => i.label === label); };
        window.showSaveDialog = async options => { suggested = options.defaultUri.fsPath; return options.defaultUri; };
        const mark = stub.calls.length;
        await stub.commands.get(command)();
        assert.deepStrictEqual(offered, expected.map(e => e[0]));
        assert.strictEqual(suggested, path.join(first, `alpha${suffix}.${ext}`));
        const read = fake.calls().filter(r => r.path === endpoint).at(-1);
        assert.strictEqual(read.query, `?format=${format}`);
        const written = since(mark, 'writeFile').at(-1);
        assert.ok(written, `${label} was not written`);
        assert.strictEqual(written[1], suggested);
        // What the server produced, byte for byte: the extension formats nothing itself.
        assert.strictEqual(Buffer.from(written[2]).toString(), `${endpoint === '/api/export' ? 'graph' : 'backpack'} of alpha as ${format}`);
      }
    } finally {
      window.showQuickPick = pick;
      window.showSaveDialog = save;
    }
  }

  // Verifies: REQ-EXT-013
  it('exports the backpack as Markdown, CSV or JSON', async () => {
    await exportEach('depphunter.exportBackpack',
      [['Markdown', 'md', 'md'], ['CSV', 'csv', 'csv'], ['JSON', 'json', 'json']],
      '-backpack', '/api/backpack');
  });

  // Verifies: REQ-EXT-014
  it('exports the graph as JSON, GraphML, DOT or HTML', async () => {
    await exportEach('depphunter.export',
      [['JSON', 'json', 'json'], ['GraphML', 'graphml', 'graphml'], ['DOT', 'dot', 'dot'], ['HTML', 'html', 'html']],
      '', '/api/export');
  });
});

// Verifies: REQ-EXT-006
describe('the rows worth picking out', () => {
  // The marks are drawn from the graph document alone, so a tree is enough here.
  const tree = new DependencyTree();
  const pkg = (id, extra) => ({ id, kind: 'package', name: id, parent: 'eco', version: '1', ...extra });
  tree.setGraph({
    root: '.', generatedAt: '', edges: [],
    nodes: [
      { id: 'eco', kind: 'ecosystem', name: 'npm' },
      pkg('plain'),
      pkg('floating', { floating: true }),
      pkg('unvouched', { indexUnknown: true, index: 'https://npm.evil.example' }),
      pkg('undeclared', { unresolved: true }),
    ],
  });
  const [eco] = tree.getChildren();
  const items = new Map(tree.getChildren(eco).map(r => [r.node.id, tree.getTreeItem(r)]));

  it('marks a package not pinned to one version in the row, with a warning colour', () => {
    const item = items.get('floating');
    assert.match(item.description, /\bfloating\b/);
    assert.strictEqual(item.iconPath.color?.id, 'list.warningForeground');
  });

  it('marks a package from an index nothing here vouches for, or that nothing declares', () => {
    assert.strictEqual(items.get('unvouched').iconPath.id, 'warning');
    assert.strictEqual(items.get('undeclared').iconPath.id, 'warning');
  });

  it('leaves an ordinary package unmarked', () => {
    const item = items.get('plain');
    assert.strictEqual(item.iconPath.id, 'package');
    assert.strictEqual(item.iconPath.color, undefined);
    assert.doesNotMatch(item.description, /floating/);
  });
});
