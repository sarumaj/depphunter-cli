// Just enough of the editor's API to run the built extension outside an editor.
//
// What this is for: everything the extension does that can break without anyone
// noticing happens between it and the depphunter binary - the arguments it starts the
// server with, and reading back the address the server prints. Neither is checked by
// the Go tests or by the type checker, and an argument depphunter stops accepting
// would only show up when somebody opened the map. So the real out/extension.js runs
// here, against a real server, with the editor stubbed out from under it.

const Module = require('node:module');

/** What the extension asked the editor to do, oldest first. */
const calls = [];

/** The settings the stub hands back; a test can change them between runs. */
const settings = {
  path: process.env.DEPPHUNTER || 'depphunter',
  watch: true,
  style: 'default',
  findings: [],
  args: [],
  openIn: 'simpleBrowser',
  editorCommand: '',
};

/** The commands the extension registered, by id. */
const commands = new Map();

let root = process.cwd();

const vscode = {
  Uri: {
    parse: s => ({ scheme: s.split(':')[0], fsPath: s, toString: () => s }),
    file: p => ({ scheme: 'file', fsPath: p, toString: () => 'file://' + p }),
  },
  env: {
    appRoot: '', // no editor to find a launcher in, so --editor is left off
    asExternalUri: async u => u,
    openExternal: async u => { calls.push(['openExternal', u.toString()]); return true; },
  },
  workspace: {
    get workspaceFolders() { return [{ uri: { fsPath: root, scheme: 'file' }, name: 'workspace' }]; },
    getConfiguration: () => ({ get: key => settings[key] }),
    getWorkspaceFolder: () => undefined,
    onDidChangeWorkspaceFolders: () => ({ dispose() {} }),
    onDidChangeConfiguration: () => ({ dispose() {} }),
  },
  window: {
    createOutputChannel: () => ({
      appendLine: s => log(s + '\n'), append: log, show() {}, dispose() {},
    }),
    createStatusBarItem: () => ({
      show() { calls.push(['status.show']); }, hide() { calls.push(['status.hide']); },
      dispose() {}, set text(v) { calls.push(['status.text', v]); },
      set tooltip(v) { calls.push(['status.tooltip', v]); }, set command(_v) {},
    }),
    showErrorMessage: async m => { calls.push(['error', m]); return undefined; },
    showInformationMessage: async m => { calls.push(['info', m]); return undefined; },
    showWorkspaceFolderPick: async () => vscode.workspace.workspaceFolders[0],
    showQuickPick: async items => items[0],
    withProgress: (_options, task) => task(
      { report() {} }, { onCancellationRequested: () => ({ dispose() {} }) }),
  },
  commands: {
    registerCommand: (id, fn) => { commands.set(id, fn); return { dispose() {} }; },
    executeCommand: async (id, ...args) => { calls.push(['executeCommand', id, ...args]); },
  },
  StatusBarAlignment: { Right: 2 },
  ProgressLocation: { Notification: 15 },
  CancellationError: class CancellationError extends Error {},
};

function log(text) {
  if (process.env.VERBOSE) process.stderr.write(text);
}

// require('vscode') resolves to nothing outside an editor, where it is injected into
// the module cache by the extension host. Do the same.
const resolve = Module._resolveFilename;
Module._resolveFilename = function (request, ...rest) {
  return request === 'vscode' ? 'vscode' : resolve.call(this, request, ...rest);
};
require.cache['vscode'] = { id: 'vscode', filename: 'vscode', loaded: true, exports: vscode };

module.exports = {
  vscode, calls, commands, settings,
  setRoot: dir => { root = dir; },
  /** The last call of a kind, or undefined. */
  last: (kind, id) => [...calls].reverse().find(c => c[0] === kind && (!id || c[1] === id)),
};
