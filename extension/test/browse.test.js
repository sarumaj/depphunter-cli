// A package's page, repository and folder, opened from the editor. No server
// needed: the tree, or the map through the server's "browse" event, hands over an
// address or a directory, and what the extension then asks of the editor is what
// the stub records.

const assert = require('node:assert');
const path = require('node:path');
const { beforeEach, describe, it } = require('node:test');

const stub = require('./stub');
const { openLink, revealFolder } = require('../out/browse.js');
const { DependencyTree } = require('../out/tree.js');

const ROOT = path.join(path.sep, 'work', 'app');
const asked = () => stub.calls.filter(c => ['openExternal', 'executeCommand', 'info'].includes(c[0]))
  .map(c => (c[0] === 'executeCommand' ? [c[1], c[2]?.fsPath, c[3]] : c));

// Verifies: REQ-EXT-037
describe('where a package can be looked into', () => {
  beforeEach(() => {
    stub.calls.length = 0;
    stub.answers.info = [];
    stub.setRoot(ROOT);
  });

  it('opens a page or a repository in the browser, and nothing that is not a web address', async () => {
    assert.strictEqual(await openLink('https://www.npmjs.com/package/lodash'), true);
    for (const url of ['javascript:alert(1)', 'file:///etc/passwd', '', undefined]) {
      assert.strictEqual(await openLink(url), false, `${url} was opened`);
    }
    assert.deepStrictEqual(asked(), [['openExternal', 'https://www.npmjs.com/package/lodash']]);
  });

  it('reveals a folder inside the workspace in the Explorer', async () => {
    await revealFolder(path.join(ROOT, 'node_modules', 'lodash'), 'lodash');
    assert.deepStrictEqual(asked(), [['revealInExplorer', path.join(ROOT, 'node_modules', 'lodash'), undefined]]);
  });

  it('offers a new window or the file manager for a folder outside it', async () => {
    const cache = path.join(path.sep, 'home', 'me', 'go', 'pkg', 'mod', 'github.com', 'pkg', 'errors@v0.9.1');
    stub.answers.info = ['Open in New Window'];
    await revealFolder(cache, 'github.com/pkg/errors');
    const [prompt, opened] = asked();
    assert.match(prompt[1], /github\.com\/pkg\/errors is installed in .*outside the workspace/);
    assert.strictEqual(prompt.length, 4, 'not two choices');
    assert.deepStrictEqual(opened, ['vscode.openFolder', cache, { forceNewWindow: true }]);

    stub.calls.length = 0;
    stub.answers.info = [prompt[3]];
    await revealFolder(cache, 'github.com/pkg/errors');
    assert.deepStrictEqual(asked()[1], ['revealFileInOS', cache, undefined]);

    stub.calls.length = 0;
    await revealFolder(cache, 'github.com/pkg/errors');
    assert.strictEqual(asked().length, 1, 'a dismissed prompt still opened something');
  });

  it('offers on each package row only the links it has', () => {
    const tree = new DependencyTree();
    tree.setGraph({
      root: 'app', generatedAt: '',
      nodes: [
        { id: 'e:npm', kind: 'ecosystem', name: 'npm' },
        { id: 'p:npm:a', kind: 'package', name: 'a', parent: 'e:npm', page: 'https://www.npmjs.com/package/a', repository: 'https://github.com/o/a' },
        { id: 'p:npm:b', kind: 'package', name: 'b', parent: 'e:npm', repository: 'https://github.com/o/b' },
        { id: 'p:npm:c', kind: 'package', name: 'c', parent: 'e:npm' },
      ],
      edges: [],
    });
    const rows = tree.getChildren(tree.getChildren().find(r => r.node.id === 'e:npm'));
    const context = Object.fromEntries(rows.map(r => [r.node.name, tree.getTreeItem(r).contextValue]));
    assert.deepStrictEqual(context, { a: 'package page repository', b: 'package repository', c: 'package' });
  });
});
