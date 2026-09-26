// A binary file from the map, opened in the Hex Editor. No server needed: the map
// asks, the server passes it on as an "open" event, and this is what the extension
// then does with it in the editor - which the stub records.

const assert = require('node:assert');
const path = require('node:path');
const { beforeEach, describe, it } = require('node:test');

const stub = require('./stub');
const { openHex } = require('../out/extension.js');

const ROOT = path.join(path.sep, 'work', 'app');
const opened = () => stub.calls.filter(c => c[0] === 'executeCommand').map(c => [c[1], c[2]?.fsPath, c[3]]);

// Verifies: REQ-EXT-034
describe('opening a binary file in the Hex Editor', () => {
  beforeEach(() => {
    stub.calls.length = 0;
    stub.installed.clear();
    stub.answers.info = [];
  });

  it('opens it with the Hex Editor when that is installed', async () => {
    stub.installed.add('ms-vscode.hexeditor');
    await openHex(ROOT, 'assets/model.bin');
    assert.deepStrictEqual(opened(), [['vscode.openWith', path.join(ROOT, 'assets', 'model.bin'), 'hexEditor.hexedit']]);
  });

  it('offers to install it, and then opens the file with it', async () => {
    stub.answers.info = ['Install Hex Editor'];
    await openHex(ROOT, 'model.bin');
    const [prompt] = stub.calls.filter(c => c[0] === 'info');
    assert.ok(prompt.includes('Install Hex Editor') && prompt.includes('Open as is'), 'the prompt did not offer both');
    assert.deepStrictEqual(opened().map(c => c[0]), ['workbench.extensions.installExtension', 'vscode.openWith']);
  });

  it('opens it as it is when asked to, and does nothing when the prompt is dismissed', async () => {
    stub.answers.info = ['Open as is'];
    await openHex(ROOT, 'model.bin');
    assert.deepStrictEqual(opened(), [['vscode.open', path.join(ROOT, 'model.bin'), undefined]]);
    stub.calls.length = 0;
    await openHex(ROOT, 'model.bin');
    assert.deepStrictEqual(opened(), [], 'a dismissed prompt still opened something');
  });
});
